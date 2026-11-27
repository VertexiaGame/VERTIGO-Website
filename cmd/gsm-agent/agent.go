package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/service/gsm"
)

const (
	failedRetention = 15 * time.Minute
	storeOpTimeout  = 3 * time.Second
)

var (
	errCapacity = errors.New(gsmagent.ErrorCapacity)
	errNotFound = errors.New("server not found")
)

// createError carries the HTTP status for a failed create.
type createError struct {
	status int
	msg    string
}

func (e *createError) Error() string { return e.msg }

type serverState struct {
	rec      record
	stopping bool
	// done is closed when the supervisor goroutine exits; nil while the
	// initial launch is still in progress.
	done chan struct{}
	// purge removes a failed record after failedRetention.
	purge *time.Timer
}

// Agent owns the gameserver processes on this host.
type Agent struct {
	cfg      agentConfig
	launcher *gsm.ProcessLauncher
	store    recordStore
	log      *slog.Logger

	mu      sync.Mutex
	ports   *portAllocator
	servers map[string]*serverState

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Tunables, overridden by tests.
	restartBackoff time.Duration
	pollInterval   time.Duration
	verifyPID      func(pid int, id string) bool
	now            func() time.Time
}

func newAgent(cfg agentConfig, store recordStore, logger *slog.Logger) *Agent {
	if logger == nil {
		logger = slog.Default()
	}
	launcher := gsm.NewProcessLauncherWithOptions(gsm.ProcessOptions{
		Bind:       cfg.BindHost,
		Bin:        cfg.ServerBin,
		Workdir:    cfg.ServerWorkdir,
		MapPath:    cfg.ServerMap,
		LogsDir:    cfg.LogsDir,
		APIKey:     cfg.APIKey,
		APIDomain:  cfg.APIDomain,
		Env:        cfg.childEnv(),
		InheritEnv: false,
		Setpgid:    true,
	})
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		cfg:            cfg,
		launcher:       launcher,
		store:          store,
		log:            logger,
		ports:          newPortAllocator(cfg.PortMin, cfg.PortMax),
		servers:        make(map[string]*serverState),
		ctx:            ctx,
		cancel:         cancel,
		restartBackoff: time.Second,
		pollInterval:   2 * time.Second,
		verifyPID:      pidMatches,
		now:            time.Now,
	}
}

// pidMatches reports whether pid is alive and its command line names the
// server id, which guards against adopting or signalling a reused pid.
func pidMatches(pid int, id string) bool {
	if pid <= 0 {
		return false
	}
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
	if err != nil || len(raw) == 0 {
		return false
	}
	for _, arg := range strings.Split(string(raw), "\x00") {
		if arg == id {
			return true
		}
	}
	return false
}

func (a *Agent) persist(r record) {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	if err := a.store.Save(ctx, r); err != nil {
		a.log.Error("persist server record failed", "server_id", r.ID, "error", err.Error())
	}
}

func (a *Agent) unpersist(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), storeOpTimeout)
	defer cancel()
	if err := a.store.Delete(ctx, id); err != nil {
		a.log.Error("delete server record failed", "server_id", id, "error", err.Error())
	}
}

func (a *Agent) activeCountLocked() int {
	n := 0
	for _, st := range a.servers {
		if st.rec.Status != gsmagent.StatusFailed {
			n++
		}
	}
	return n
}

func (a *Agent) capacityLocked() int {
	free := a.ports.Free()
	if a.cfg.MaxServers > 0 {
		if left := a.cfg.MaxServers - a.activeCountLocked(); left < free {
			free = left
		}
	}
	if free < 0 {
		free = 0
	}
	return free
}

// removeLocked forgets a server and releases its port.
func (a *Agent) removeLocked(st *serverState) {
	if cur, ok := a.servers[st.rec.ID]; ok && cur == st {
		delete(a.servers, st.rec.ID)
	}
	if st.purge != nil {
		st.purge.Stop()
	}
	a.ports.Release(st.rec.Port, st.rec.ID)
	a.unpersist(st.rec.ID)
}

// Create launches a gameserver and waits for readiness.
func (a *Agent) Create(req gsmagent.CreateServerRequest) (gsmagent.Server, error) {
	mapPath, err := validateCreate(req, a.cfg.ServerMap, a.cfg.MapDir)
	if err != nil {
		return gsmagent.Server{}, &createError{status: 400, msg: err.Error()}
	}
	a.mu.Lock()
	if a.ctx.Err() != nil {
		a.mu.Unlock()
		return gsmagent.Server{}, &createError{status: 503, msg: "agent is shutting down"}
	}
	if st, ok := a.servers[req.ServerID]; ok {
		view := st.rec.view()
		a.mu.Unlock()
		return view, &createError{status: 409, msg: "server id already exists"}
	}
	if a.cfg.MaxServers > 0 && a.activeCountLocked() >= a.cfg.MaxServers {
		a.mu.Unlock()
		return gsmagent.Server{}, errCapacity
	}
	port, ok := a.ports.Claim(req.ServerID)
	if !ok {
		a.mu.Unlock()
		return gsmagent.Server{}, errCapacity
	}
	st := &serverState{rec: record{
		ID:         req.ServerID,
		GameID:     req.GameID,
		Map:        req.Map,
		MapPath:    mapPath,
		MaxPlayers: req.MaxPlayers,
		Port:       port,
		Status:     gsmagent.StatusStarting,
		CreatedAt:  a.now(),
	}}
	a.servers[req.ServerID] = st
	a.persist(st.rec)
	a.mu.Unlock()

	log := a.log.With("server_id", req.ServerID, "game_id", req.GameID, "port", port)
	log.Info("launching gameserver")
	pid, err := a.launcher.LaunchMap(req.ServerID, req.GameID, port, mapPath)
	if err != nil {
		a.mu.Lock()
		a.removeLocked(st)
		a.mu.Unlock()
		log.Error("gameserver launch failed", "error", err.Error())
		return gsmagent.Server{}, &createError{status: 503, msg: "launch failed: " + err.Error()}
	}
	a.mu.Lock()
	st.rec.PID = pid
	a.persist(st.rec)
	stopping := st.stopping
	a.mu.Unlock()

	if !stopping {
		err = a.launcher.WaitHealthy(req.ServerID, port, pid, a.cfg.ReadyTimeout)
	}
	a.mu.Lock()
	if stopping || st.stopping {
		err = errors.New("stopped during startup")
	}
	if err != nil {
		a.mu.Unlock()
		_ = a.launcher.Stop(pid, a.cfg.ShutdownGrace)
		a.mu.Lock()
		a.removeLocked(st)
		a.mu.Unlock()
		log.Error("gameserver did not become ready", "pid", pid, "error", err.Error())
		return gsmagent.Server{}, &createError{status: 503, msg: err.Error()}
	}
	st.rec.Status = gsmagent.StatusRunning
	st.rec.StartedAt = a.now()
	a.persist(st.rec)
	a.startSupervisorLocked(st, pid)
	view := st.rec.view()
	a.mu.Unlock()
	log.Info("gameserver ready", "pid", pid)
	return view, nil
}

// Get returns one server.
func (a *Agent) Get(id string) (gsmagent.Server, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	st, ok := a.servers[id]
	if !ok {
		return gsmagent.Server{}, false
	}
	return st.rec.view(), true
}

// List returns every server, oldest first.
func (a *Agent) List() []gsmagent.Server {
	a.mu.Lock()
	out := make([]gsmagent.Server, 0, len(a.servers))
	created := make(map[string]time.Time, len(a.servers))
	for id, st := range a.servers {
		out = append(out, st.rec.view())
		created[id] = st.rec.CreatedAt
	}
	a.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if !created[out[i].ID].Equal(created[out[j].ID]) {
			return created[out[i].ID].Before(created[out[j].ID])
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Health summarizes the agent.
func (a *Agent) Health() gsmagent.Health {
	a.mu.Lock()
	defer a.mu.Unlock()
	return gsmagent.Health{
		OK:       a.ctx.Err() == nil,
		Servers:  a.activeCountLocked(),
		Capacity: a.capacityLocked(),
		Version:  version,
		AgentID:  a.cfg.AgentID,
	}
}

// Stop terminates a server: SIGTERM to its process group, SIGKILL after the
// shutdown grace, then the record and port are released.
func (a *Agent) Stop(id string) error {
	a.mu.Lock()
	st, ok := a.servers[id]
	if !ok {
		a.mu.Unlock()
		return errNotFound
	}
	if st.rec.Status == gsmagent.StatusFailed {
		a.removeLocked(st)
		a.mu.Unlock()
		return nil
	}
	st.stopping = true
	st.rec.Status = gsmagent.StatusStopping
	a.persist(st.rec)
	pid := st.rec.PID
	done := st.done
	a.mu.Unlock()

	log := a.log.With("server_id", id, "pid", pid)
	log.Info("stopping gameserver")
	if pid > 0 && (a.launcher.Owns(pid) || a.verifyPID(pid, id)) {
		_ = a.launcher.Stop(pid, a.cfg.ShutdownGrace)
	}
	if done == nil {
		// The initial launch is still in flight; Create cleans up.
		return nil
	}
	select {
	case <-done:
	case <-time.After(a.cfg.ShutdownGrace + a.cfg.ReadyTimeout + 5*time.Second):
		log.Warn("supervisor did not exit in time")
	}
	a.mu.Lock()
	a.removeLocked(st)
	a.mu.Unlock()
	return nil
}

// exitSignal returns a channel closed when pid exits. Children of this
// process are tracked by the launcher; adopted processes are polled.
func (a *Agent) exitSignal(pid int, id string) <-chan struct{} {
	if pid <= 0 {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	if ch := a.launcher.Exited(pid); ch != nil {
		return ch
	}
	ch := make(chan struct{})
	go func() {
		t := time.NewTicker(a.pollInterval)
		defer t.Stop()
		for {
			if !a.verifyPID(pid, id) {
				close(ch)
				return
			}
			select {
			case <-a.ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return ch
}

func (a *Agent) startSupervisorLocked(st *serverState, pid int) {
	if a.ctx.Err() != nil {
		// Shutting down: the persisted record lets the next run adopt it.
		return
	}
	st.done = make(chan struct{})
	a.wg.Add(1)
	go a.supervise(st, pid)
}

func pruneRestarts(times []time.Time, now time.Time, window time.Duration) []time.Time {
	kept := times[:0]
	for _, t := range times {
		if now.Sub(t) < window {
			kept = append(kept, t)
		}
	}
	return kept
}

// supervise watches one server. An unexpected exit restarts it on the same
// port, up to MaxRestarts within RestartWindow; then it is marked failed and
// its port released. When the agent shuts down the supervisor returns and
// leaves the process running for the next agent to adopt.
func (a *Agent) supervise(st *serverState, pid int) {
	defer a.wg.Done()
	defer close(st.done)
	id := st.rec.ID
	log := a.log.With("server_id", id, "port", st.rec.Port)
	for {
		exited := a.exitSignal(pid, id)
		select {
		case <-a.ctx.Done():
			return
		case <-exited:
		}
		if pid > 0 {
			a.launcher.Forget(pid)
		}
		a.mu.Lock()
		if st.stopping {
			a.mu.Unlock()
			return
		}
		now := a.now()
		st.rec.RestartTimes = pruneRestarts(st.rec.RestartTimes, now, a.cfg.RestartWindow)
		if len(st.rec.RestartTimes) >= a.cfg.MaxRestarts {
			a.failLocked(st, fmt.Sprintf("exited %d times within %s", len(st.rec.RestartTimes)+1, a.cfg.RestartWindow))
			restarts := st.rec.Restarts
			a.mu.Unlock()
			log.Error("gameserver failed permanently; port released", "restarts", restarts, "log_tail", gsm.LogTail(a.launcher.LogPath(id)))
			return
		}
		st.rec.RestartTimes = append(st.rec.RestartTimes, now)
		st.rec.Restarts++
		st.rec.Status = gsmagent.StatusRestarting
		st.rec.PID = 0
		attempt := len(st.rec.RestartTimes)
		a.persist(st.rec)
		rec := st.rec
		a.mu.Unlock()
		log.Warn("gameserver exited unexpectedly; restarting", "old_pid", pid, "attempt", attempt, "log_tail", gsm.LogTail(a.launcher.LogPath(id)))

		select {
		case <-a.ctx.Done():
			return
		case <-time.After(a.restartBackoff * time.Duration(attempt)):
		}
		a.mu.Lock()
		stopping := st.stopping
		a.mu.Unlock()
		if stopping {
			return
		}

		newPID, err := a.launcher.LaunchMap(id, rec.GameID, rec.Port, rec.MapPath)
		if err == nil {
			a.mu.Lock()
			st.rec.PID = newPID
			a.persist(st.rec)
			a.mu.Unlock()
			if err = a.launcher.WaitHealthy(id, rec.Port, newPID, a.cfg.ReadyTimeout); err != nil {
				_ = a.launcher.Stop(newPID, a.cfg.ShutdownGrace)
			}
		}
		a.mu.Lock()
		if st.stopping {
			a.mu.Unlock()
			if err == nil {
				_ = a.launcher.Stop(newPID, a.cfg.ShutdownGrace)
			}
			return
		}
		if err != nil {
			st.rec.PID = 0
			st.rec.LastError = err.Error()
			a.persist(st.rec)
			a.mu.Unlock()
			log.Error("gameserver restart failed", "error", err.Error())
			pid = 0
			continue
		}
		st.rec.Status = gsmagent.StatusRunning
		st.rec.StartedAt = a.now()
		st.rec.LastError = ""
		a.persist(st.rec)
		a.mu.Unlock()
		log.Info("gameserver restarted", "pid", newPID, "restarts", rec.Restarts)
		pid = newPID
	}
}

// failLocked marks a server failed, releases its port and schedules removal
// of the record so the platform can still observe the failure for a while.
func (a *Agent) failLocked(st *serverState, reason string) {
	st.rec.Status = gsmagent.StatusFailed
	st.rec.PID = 0
	st.rec.FailedAt = a.now()
	st.rec.LastError = reason
	a.ports.Release(st.rec.Port, st.rec.ID)
	a.persist(st.rec)
	a.schedulePurgeLocked(st, failedRetention)
}

func (a *Agent) schedulePurgeLocked(st *serverState, after time.Duration) {
	if st.purge != nil {
		st.purge.Stop()
	}
	st.purge = time.AfterFunc(after, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		if cur, ok := a.servers[st.rec.ID]; ok && cur == st && st.rec.Status == gsmagent.StatusFailed {
			delete(a.servers, st.rec.ID)
			a.unpersist(st.rec.ID)
		}
	})
}

// Reconcile loads persisted records after an agent restart: running
// processes are adopted, dead ones restarted (counting as a restart) and
// failed ones keep their port released.
func (a *Agent) Reconcile(ctx context.Context) error {
	recs, err := a.store.LoadAll(ctx)
	if err != nil {
		return err
	}
	now := a.now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, rec := range recs {
		log := a.log.With("server_id", rec.ID, "port", rec.Port, "pid", rec.PID)
		if !gsm.ValidID(rec.ID) {
			a.unpersist(rec.ID)
			continue
		}
		alive := rec.PID > 0 && a.verifyPID(rec.PID, rec.ID)
		switch rec.Status {
		case gsmagent.StatusFailed:
			if now.Sub(rec.FailedAt) >= failedRetention {
				a.unpersist(rec.ID)
				continue
			}
			st := &serverState{rec: rec}
			a.servers[rec.ID] = st
			a.schedulePurgeLocked(st, failedRetention-now.Sub(rec.FailedAt))
			continue
		case gsmagent.StatusStopping:
			if alive {
				_ = a.launcher.Stop(rec.PID, a.cfg.ShutdownGrace)
			}
			a.unpersist(rec.ID)
			log.Info("finished interrupted stop")
			continue
		}
		if !a.ports.ClaimSpecific(rec.Port, rec.ID) {
			if alive {
				_ = a.launcher.Stop(rec.PID, a.cfg.ShutdownGrace)
			}
			a.unpersist(rec.ID)
			log.Warn("dropping record whose port is outside the range or already taken")
			continue
		}
		st := &serverState{rec: rec}
		a.servers[rec.ID] = st
		if alive {
			st.rec.Status = gsmagent.StatusRunning
			a.persist(st.rec)
			a.startSupervisorLocked(st, rec.PID)
			log.Info("adopted running gameserver")
			continue
		}
		log.Warn("gameserver from previous agent run is gone; restarting")
		a.startSupervisorLocked(st, 0)
	}
	return nil
}

// Close stops supervising without touching gameserver processes, so a
// restarted agent can adopt them.
func (a *Agent) Close(timeout time.Duration) {
	a.mu.Lock()
	a.cancel()
	a.mu.Unlock()
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		a.log.Warn("supervisors still busy at shutdown; leaving them")
	}
}
