package gsm

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"vertexia-frontend/backend/gsmagent"
)

var (
	ErrUnavailable = errors.New("gameserver manager unavailable")
	ErrGameMissing = errors.New("game not found")
	ErrGameClosed  = errors.New("game is not joinable")
	ErrNoCapacity  = errors.New("no gameserver capacity available")
	ErrLaunch      = errors.New("could not launch gameserver")
	ErrMissing     = errors.New("gameserver not found")
	ErrFull        = errors.New("gameserver is full")
)

const (
	startGrace   = 2 * time.Minute
	joinAttempts = 3
	// orphanGrace protects agent servers whose platform record may still be
	// in flight from the orphan sweep.
	orphanGrace         = time.Minute
	defaultReadyTimeout = 30 * time.Second
	defaultAgentTimeout = 10 * time.Second
)

// RemoteAgent is the subset of *gsmagent.Client the manager needs.
type RemoteAgent interface {
	CreateServer(ctx context.Context, req gsmagent.CreateServerRequest) (gsmagent.Server, error)
	ListServers(ctx context.Context) ([]gsmagent.Server, error)
	StopServer(ctx context.Context, id string) error
	Health(ctx context.Context) (gsmagent.Health, error)
}

// childStopper is implemented by launchers that can gracefully stop the
// children they started (ProcessLauncher).
type childStopper interface {
	Owns(pid int) bool
	Stop(pid int, grace time.Duration) error
	StopAll(grace time.Duration) []int
}

type Manager struct {
	cfg      Config
	store    Store
	launcher Launcher
	agent    RemoteAgent
	resolve  GameResolver
	locks    locker

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewManager builds a manager that launches gameservers as local children.
func NewManager(cfg Config, store Store, launcher Launcher, resolve GameResolver) *Manager {
	if store == nil {
		store = NewMemoryStore()
	}
	return &Manager{
		cfg:      cfg,
		store:    store,
		launcher: launcher,
		resolve:  resolve,
		locks:    newLocker(store),
		stopCh:   make(chan struct{}),
	}
}

// NewRemoteManager builds a manager that drives a gsm-agent on a dedicated
// gameserver host. The agent allocates ports; the manager never claims ports
// from its own range in this mode.
func NewRemoteManager(cfg Config, store Store, agent RemoteAgent, resolve GameResolver) *Manager {
	m := NewManager(cfg, store, nil, resolve)
	m.agent = agent
	return m
}

// NewAgentClient builds the signed agent client from cfg.
func NewAgentClient(cfg Config) (*gsmagent.Client, error) {
	timeout := cfg.AgentTimeout
	if timeout <= 0 {
		timeout = defaultAgentTimeout
	}
	return gsmagent.NewClient(cfg.AgentURL, []byte(cfg.AgentSecret), timeout)
}

func (m *Manager) Store() Store {
	return m.store
}

func (m *Manager) Config() Config {
	if m == nil {
		return Config{}
	}
	return m.cfg
}

// Remote reports whether gameservers are managed by a remote agent.
func (m *Manager) Remote() bool {
	return m != nil && m.agent != nil
}

func (m *Manager) ready() bool {
	return m != nil && m.store != nil && (m.launcher != nil || m.agent != nil)
}

func (m *Manager) readyTimeout() time.Duration {
	if m.cfg.ReadyTimeout > 0 {
		return m.cfg.ReadyTimeout
	}
	return defaultReadyTimeout
}

func (m *Manager) agentTimeout() time.Duration {
	if m.cfg.AgentTimeout > 0 {
		return m.cfg.AgentTimeout
	}
	return defaultAgentTimeout
}

// spawnLockTTL covers the longest a join can hold the per-game lock: a full
// launch plus a failed-launch stop.
func (m *Manager) spawnLockTTL() time.Duration {
	return m.readyTimeout() + 2*m.agentTimeout() + m.cfg.ShutdownGrace + 15*time.Second
}

func gameLockKey(gameID int) string {
	return keyLockGame + strconv.Itoa(gameID)
}

func (m *Manager) remoteAlive(ctx context.Context) (map[string]gsmagent.Server, error) {
	list, err := m.agent.ListServers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]gsmagent.Server, len(list))
	for _, srv := range list {
		out[srv.ID] = srv
	}
	return out, nil
}

func (m *Manager) Liveness() (map[string]bool, error) {
	if !m.ready() {
		return nil, ErrUnavailable
	}
	var remote map[string]gsmagent.Server
	if m.agent != nil {
		ctx, cancel := context.WithTimeout(context.Background(), m.agentTimeout())
		defer cancel()
		var err error
		if remote, err = m.remoteAlive(ctx); err != nil {
			return nil, err
		}
	}
	servers, err := m.store.ListAll()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(servers))
	for _, gs := range servers {
		if m.agent != nil {
			srv, ok := remote[gs.ID]
			out[gs.ID] = ok && srv.Alive()
			continue
		}
		out[gs.ID] = m.launcher.Alive(gs.PID, gs.Port)
	}
	return out, nil
}

func (m *Manager) effectiveMax(gameMax int) int {
	lower := m.cfg.MinPlayersDefault
	if lower < 2 {
		lower = 2
	}
	upper := m.cfg.MaxPlayersDefault
	if upper <= 0 {
		upper = 100
	}
	if gameMax <= 0 {
		return upper
	}
	if gameMax < lower {
		return lower
	}
	if gameMax > upper {
		return upper
	}
	return gameMax
}

func (m *Manager) pickCandidate(servers []GameServer, max int, now time.Time) (GameServer, bool) {
	cands := make([]GameServer, 0, len(servers))
	for _, gs := range servers {
		if gs.PlayerCount >= max {
			continue
		}
		// A remote launch still in flight has no port yet.
		if gs.Port <= 0 {
			continue
		}
		if !gs.IsJoinable(now) {
			continue
		}
		cands = append(cands, gs)
	}
	if len(cands) == 0 {
		return GameServer{}, false
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].PlayerCount != cands[j].PlayerCount {
			return cands[i].PlayerCount < cands[j].PlayerCount
		}
		return cands[i].CreatedAt.Before(cands[j].CreatedAt)
	})
	return cands[0], true
}

func (m *Manager) Join(gameID int, member string) (GameServer, bool, error) {
	return m.JoinWithProgress(gameID, member, nil)
}

func (m *Manager) JoinWithProgress(gameID int, member string, progress func(string)) (GameServer, bool, error) {
	if !m.ready() {
		return GameServer{}, false, ErrUnavailable
	}
	info, err := m.resolve(gameID)
	if err != nil {
		return GameServer{}, false, ErrGameMissing
	}
	if info.Locked || (info.Private && member != info.Owner) {
		return GameServer{}, false, ErrGameClosed
	}
	// One spawn decision per game at a time, across every platform instance
	// when the store is Redis.
	ttl := m.spawnLockTTL()
	lockCtx, cancel := context.WithTimeout(context.Background(), ttl)
	unlock, err := m.locks.Lock(lockCtx, gameLockKey(gameID), ttl)
	cancel()
	if err != nil {
		return GameServer{}, false, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer unlock()
	max := m.effectiveMax(info.MaxPlayers)
	now := time.Now()
	for i := 0; i < joinAttempts; i++ {
		servers, err := m.store.ListByGame(gameID)
		if err != nil {
			return GameServer{}, false, err
		}
		cand, ok := m.pickCandidate(servers, max, now)
		if !ok {
			break
		}
		count, joined, _, err := m.store.TryJoin(cand.ID, member, max)
		if err != nil {
			return GameServer{}, false, err
		}
		if !joined {
			continue
		}
		gs, found, err := m.store.Get(cand.ID)
		if err != nil {
			return GameServer{}, false, err
		}
		if !found {
			continue
		}
		gs.PlayerCount = count
		gs.MaxPlayers = max
		gs.LastBeat = now
		gs.EmptySince = time.Time{}
		_ = m.store.Save(gs)
		gs.Host = m.cfg.PublicHost
		return gs, true, nil
	}
	if progress != nil {
		progress("starting")
	}
	gs, err := m.spawn(gameID, max, member)
	if err != nil {
		return GameServer{}, false, err
	}
	return gs, false, nil
}

func (m *Manager) freshID() (string, error) {
	for i := 0; i < 10; i++ {
		id, err := GenerateID()
		if err != nil {
			return "", err
		}
		exists, err := m.store.Exists(id)
		if err != nil {
			return "", err
		}
		if !exists {
			return id, nil
		}
	}
	return "", errors.New("could not allocate gameserver id")
}

func (m *Manager) freePort() (int, error) {
	for port := m.cfg.PortMin; port <= m.cfg.PortMax; port++ {
		ok, err := m.store.ClaimPort(port)
		if err != nil {
			return 0, err
		}
		if !ok {
			continue
		}
		return port, nil
	}
	return 0, ErrNoCapacity
}

// reserve saves the starting record, enforcing MaxServers atomically across
// instances by counting and saving under the capacity lock.
func (m *Manager) reserve(gs GameServer) error {
	if m.cfg.MaxServers <= 0 {
		return m.store.Save(gs)
	}
	const ttl = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	unlock, err := m.locks.Lock(ctx, keyLockCapacity, ttl)
	cancel()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer unlock()
	all, err := m.store.ListAll()
	if err != nil {
		return err
	}
	if len(all) >= m.cfg.MaxServers {
		return ErrNoCapacity
	}
	return m.store.Save(gs)
}

func (m *Manager) spawn(gameID, max int, member string) (GameServer, error) {
	if m.agent != nil {
		return m.spawnRemote(gameID, max, member)
	}
	return m.spawnLocal(gameID, max, member)
}

func (m *Manager) spawnLocal(gameID, max int, member string) (GameServer, error) {
	id, err := m.freshID()
	if err != nil {
		return GameServer{}, err
	}
	port, err := m.freePort()
	if err != nil {
		return GameServer{}, err
	}
	now := time.Now()
	gs := GameServer{
		ID:         id,
		GameID:     gameID,
		Host:       m.cfg.PublicHost,
		Port:       port,
		Status:     StatusStarting,
		MaxPlayers: max,
		CreatedAt:  now,
		LastBeat:   now,
	}
	if err := m.reserve(gs); err != nil {
		_ = m.store.ReleasePort(port)
		return GameServer{}, err
	}
	log.Printf("GSM spawn: launching server %s for game %d on port %d", id, gameID, port)
	pid, err := m.launcher.Launch(id, gameID, port)
	if err != nil {
		_ = m.store.Delete(id)
		_ = m.store.ReleasePort(port)
		log.Printf("GSM spawn %s failed at launch: %v", id, err)
		return GameServer{}, fmt.Errorf("%w: launch failed: %v", ErrLaunch, err)
	}
	gs.PID = pid
	_ = m.store.Save(gs)
	if err := m.launcher.WaitHealthy(id, port, pid, m.readyTimeout()); err != nil {
		_ = m.launcher.Kill(pid, port)
		_ = m.store.Delete(id)
		_ = m.store.ReleasePort(port)
		log.Printf("GSM spawn %s pid %d port %d never became healthy: %v", id, pid, port, err)
		return GameServer{}, fmt.Errorf("%w: server did not start: %v", ErrLaunch, err)
	}
	return m.finishSpawn(gs, member, max)
}

func (m *Manager) spawnRemote(gameID, max int, member string) (GameServer, error) {
	id, err := m.freshID()
	if err != nil {
		return GameServer{}, err
	}
	now := time.Now()
	// Port stays 0 until the agent reports the one it allocated; records
	// without a port are never offered to joiners.
	gs := GameServer{
		ID:         id,
		GameID:     gameID,
		Host:       m.cfg.PublicHost,
		Status:     StatusStarting,
		MaxPlayers: max,
		CreatedAt:  now,
		LastBeat:   now,
	}
	if err := m.reserve(gs); err != nil {
		return GameServer{}, err
	}
	log.Printf("GSM spawn: requesting server %s for game %d from agent", id, gameID)
	ctx, cancel := context.WithTimeout(context.Background(), m.readyTimeout()+m.agentTimeout())
	srv, err := m.agent.CreateServer(ctx, gsmagent.CreateServerRequest{ServerID: id, GameID: gameID, MaxPlayers: max})
	cancel()
	if errors.Is(err, gsmagent.ErrExists) && srv.ID == id && srv.GameID == gameID && srv.Alive() {
		err = nil
	}
	if err != nil {
		_ = m.store.Delete(id)
		switch {
		case errors.Is(err, gsmagent.ErrCapacity):
			log.Printf("GSM spawn %s: agent has no capacity", id)
			return GameServer{}, ErrNoCapacity
		case errors.Is(err, gsmagent.ErrExists):
			log.Printf("GSM spawn %s: id already used on agent by another server", id)
		default:
			// The agent may still bring the server up after we gave up.
			if serr := m.stopRemote(id); serr != nil {
				log.Printf("GSM spawn %s: cleanup stop failed: %v", id, serr)
			}
		}
		log.Printf("GSM spawn %s failed on agent: %v", id, err)
		return GameServer{}, fmt.Errorf("%w: %v", ErrLaunch, err)
	}
	if srv.Port <= 0 {
		_ = m.stopRemote(id)
		_ = m.store.Delete(id)
		return GameServer{}, fmt.Errorf("%w: agent returned no port", ErrLaunch)
	}
	gs.Port = srv.Port
	gs.PID = srv.PID
	if err := m.store.Save(gs); err != nil {
		_ = m.stopRemote(id)
		_ = m.store.Delete(id)
		return GameServer{}, err
	}
	return m.finishSpawn(gs, member, max)
}

func (m *Manager) finishSpawn(gs GameServer, member string, max int) (GameServer, error) {
	count, joined, _, err := m.store.TryJoin(gs.ID, member, max)
	if err != nil || !joined {
		m.cleanup(gs)
		if err != nil {
			return GameServer{}, err
		}
		log.Printf("GSM spawn %s lost its join race right after launch", gs.ID)
		return GameServer{}, fmt.Errorf("%w: join race lost", ErrLaunch)
	}
	gs.Status = StatusActive
	gs.PlayerCount = count
	gs.LastBeat = time.Now()
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	log.Printf("GSM spawn: server %s for game %d active on port %d pid %d", gs.ID, gs.GameID, gs.Port, gs.PID)
	return gs, nil
}

func (m *Manager) Servers(gameID int) ([]GameServer, error) {
	if m == nil || m.store == nil {
		return nil, ErrUnavailable
	}
	servers, err := m.store.ListByGame(gameID)
	if err != nil {
		return nil, err
	}
	out := make([]GameServer, 0, len(servers))
	for _, gs := range servers {
		if gs.Status == StatusStarting || gs.Status == StatusActive || gs.Status == StatusDraining {
			gs.Host = m.cfg.PublicHost
			out = append(out, gs)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (m *Manager) ListAll() ([]GameServer, error) {
	if m == nil || m.store == nil {
		return nil, ErrUnavailable
	}
	servers, err := m.store.ListAll()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		servers[i].Host = m.cfg.PublicHost
	}
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].CreatedAt.Before(servers[j].CreatedAt)
	})
	return servers, nil
}

func (m *Manager) Leave(id string, member string) (GameServer, error) {
	if m == nil || m.store == nil {
		return GameServer{}, ErrUnavailable
	}
	count, err := m.store.Leave(id, member)
	if err != nil {
		return GameServer{}, err
	}
	_, _ = m.store.Unconfirm(id, member)
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	gs.PlayerCount = count
	if count == 0 && gs.EmptySince.IsZero() {
		gs.EmptySince = time.Now()
	}
	gs.LastBeat = time.Now()
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	return gs, nil
}

func (m *Manager) Get(id string) (GameServer, error) {
	if m == nil || m.store == nil {
		return GameServer{}, ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	gs.Host = m.cfg.PublicHost
	return gs, nil
}

func (m *Manager) EffectiveMaxForGame(gameMax int) int {
	if m == nil {
		return gameMax
	}
	return m.effectiveMax(gameMax)
}

func (m *Manager) ServerJoin(id string, member string) (GameServer, bool, error) {
	if !m.ready() {
		return GameServer{}, false, ErrUnavailable
	}
	if member == "" {
		return GameServer{}, false, ErrMissing
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, false, err
	}
	if !found {
		return GameServer{}, false, ErrMissing
	}
	info, err := m.resolve(gs.GameID)
	if err != nil {
		return GameServer{}, false, ErrGameMissing
	}
	if info.Locked || (info.Private && member != info.Owner) {
		return GameServer{}, false, ErrGameClosed
	}
	max := m.effectiveMax(info.MaxPlayers)
	now := time.Now()
	count, joined, isNew, err := m.store.TryJoin(id, member, max)
	if err != nil {
		return GameServer{}, false, err
	}
	if !joined {
		return GameServer{}, false, ErrFull
	}
	gs, found, err = m.store.Get(id)
	if err != nil {
		return GameServer{}, false, err
	}
	if !found {
		return GameServer{}, false, ErrMissing
	}
	gs.PlayerCount = count
	gs.MaxPlayers = max
	gs.LastBeat = now
	gs.LastHeartbeat = now
	gs.EmptySince = time.Time{}
	if gs.Status == StatusStarting {
		gs.Status = StatusActive
	}
	_ = m.store.Save(gs)
	_, _ = m.store.Confirm(id, member)
	gs.Host = m.cfg.PublicHost
	return gs, isNew, nil
}

func (m *Manager) SyncRoster(id string, members []string) (GameServer, error) {
	if m == nil || m.store == nil {
		return GameServer{}, ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	max := m.cfg.MaxPlayersDefault
	if max <= 0 {
		max = 100
	}
	if m.resolve != nil {
		if info, err := m.resolve(gs.GameID); err == nil {
			max = m.effectiveMax(info.MaxPlayers)
		}
	}
	seen := make(map[string]struct{}, len(members))
	wanted := make([]string, 0, len(members))
	for _, member := range members {
		if _, ok := seen[member]; ok || member == "" {
			continue
		}
		seen[member] = struct{}{}
		wanted = append(wanted, member)
	}
	sort.Strings(wanted)
	if len(wanted) > max {
		wanted = wanted[:max]
	}
	want := make(map[string]struct{}, len(wanted))
	for _, member := range wanted {
		want[member] = struct{}{}
	}
	current, err := m.store.Members(id)
	if err != nil {
		return GameServer{}, err
	}
	for _, member := range current {
		if _, ok := want[member]; !ok {
			_, _ = m.store.Leave(id, member)
		}
	}
	for _, member := range wanted {
		_, _, _, _ = m.store.TryJoin(id, member, max)
	}
	finals, err := m.store.Members(id)
	if err != nil {
		return GameServer{}, err
	}
	present := make([]string, 0, len(wanted))
	for _, member := range finals {
		if _, ok := want[member]; ok {
			present = append(present, member)
		}
	}
	_, _ = m.store.SetConfirmed(id, present)
	gs, found, err = m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	now := time.Now()
	if gs.PlayerCount > 0 {
		gs.EmptySince = time.Time{}
	} else if gs.EmptySince.IsZero() {
		gs.EmptySince = now
	}
	gs.MaxPlayers = max
	gs.LastBeat = now
	gs.LastHeartbeat = now
	if gs.Status == StatusStarting {
		gs.Status = StatusActive
	}
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	return gs, nil
}

func (m *Manager) Heartbeat(id string) (GameServer, error) {
	if m == nil || m.store == nil {
		return GameServer{}, ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	now := time.Now()
	if gs.PlayerCount > 0 {
		gs.EmptySince = time.Time{}
	} else if gs.EmptySince.IsZero() {
		gs.EmptySince = now
	}
	gs.LastBeat = now
	gs.LastHeartbeat = now
	if gs.Status == StatusStarting {
		gs.Status = StatusActive
	}
	if m.resolve != nil {
		if info, err := m.resolve(gs.GameID); err == nil {
			gs.MaxPlayers = m.effectiveMax(info.MaxPlayers)
		}
	}
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	return gs, nil
}

func (m *Manager) Drain(id string) (GameServer, error) {
	if m == nil || m.store == nil {
		return GameServer{}, ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return GameServer{}, err
	}
	if !found {
		return GameServer{}, ErrMissing
	}
	now := time.Now()
	gs.Status = StatusDraining
	gs.DrainingAt = now
	gs.LastBeat = now
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	return gs, nil
}

func (m *Manager) Kill(id string) error {
	if !m.ready() {
		return ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return err
	}
	if !found {
		return ErrMissing
	}
	m.cleanup(gs)
	return nil
}

func (m *Manager) stopRemote(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), m.agentTimeout()+m.cfg.ShutdownGrace+5*time.Second)
	defer cancel()
	err := m.agent.StopServer(ctx, id)
	if errors.Is(err, gsmagent.ErrNotFound) {
		return nil
	}
	return err
}

// cleanup stops the server and removes its record. In remote mode a failed
// stop is retried by the orphan sweep in ReapOnce, since the record is gone.
func (m *Manager) cleanup(gs GameServer) {
	if m.agent != nil {
		if err := m.stopRemote(gs.ID); err != nil {
			log.Printf("GSM stop %s via agent failed (orphan sweep will retry): %v", gs.ID, err)
		}
		_ = m.store.Delete(gs.ID)
		return
	}
	_ = m.launcher.Kill(gs.PID, gs.Port)
	_ = m.store.Delete(gs.ID)
	_ = m.store.ReleasePort(gs.Port)
}

func (m *Manager) heartbeatExpired(gs GameServer, now time.Time) bool {
	if m.cfg.HeartbeatTimeout <= 0 || gs.Status != StatusActive {
		return false
	}
	ref := gs.LastHeartbeat
	if ref.IsZero() {
		ref = gs.CreatedAt
	}
	return !ref.IsZero() && now.Sub(ref) > m.cfg.HeartbeatTimeout
}

// ReapOnce runs one maintenance sweep: it removes dead servers, evicts
// active servers that stopped heartbeating, winds idle servers down
// (active -> draining -> stopped) and, in remote mode, stops agent servers
// that have no platform record. Remote liveness costs one ListServers call.
func (m *Manager) ReapOnce() ([]string, error) {
	if !m.ready() {
		return nil, ErrUnavailable
	}
	var remote map[string]gsmagent.Server
	remoteOK := false
	if m.agent != nil {
		// List the agent before the store: every agent server has a record
		// saved before its creation, so anything missing from the later
		// store snapshot is a genuine orphan.
		ctx, cancel := context.WithTimeout(context.Background(), m.agentTimeout())
		var err error
		remote, err = m.remoteAlive(ctx)
		cancel()
		if err != nil {
			log.Printf("GSM reaper: agent unreachable, skipping liveness checks: %v", err)
		} else {
			remoteOK = true
		}
	}
	servers, err := m.store.ListAll()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var reaped []string
	known := make(map[string]struct{}, len(servers))
	for _, gs := range servers {
		known[gs.ID] = struct{}{}
		var alive bool
		switch {
		case m.agent == nil:
			alive = m.launcher.Alive(gs.PID, gs.Port)
		case !remoteOK:
			alive = true
		default:
			srv, ok := remote[gs.ID]
			alive = ok && srv.Alive()
			if alive && gs.Port > 0 && srv.PID > 0 && srv.PID != gs.PID {
				gs.PID = srv.PID
				_ = m.store.Save(gs)
			}
		}
		if !alive {
			if gs.Status == StatusStarting && now.Sub(gs.CreatedAt) < startGrace {
				continue
			}
			m.cleanup(gs)
			reaped = append(reaped, gs.ID)
			continue
		}
		if m.heartbeatExpired(gs, now) {
			log.Printf("GSM evicting %s: no heartbeat for %s", gs.ID, m.cfg.HeartbeatTimeout)
			m.cleanup(gs)
			reaped = append(reaped, gs.ID)
			continue
		}
		switch gs.Status {
		case StatusStarting:
			if gs.Port <= 0 {
				// Remote launch in flight; the spawner finalizes it. A record
				// still portless after the start grace lost its spawner.
				if now.Sub(gs.CreatedAt) >= startGrace {
					m.cleanup(gs)
					reaped = append(reaped, gs.ID)
				}
				continue
			}
			gs.Status = StatusActive
			gs.LastBeat = now
			_ = m.store.Save(gs)
		case StatusDraining:
			if gs.DrainingAt.IsZero() {
				gs.DrainingAt = now
				_ = m.store.Save(gs)
				continue
			}
			if now.Sub(gs.DrainingAt) >= m.cfg.ShutdownGrace {
				m.cleanup(gs)
				reaped = append(reaped, gs.ID)
			}
		default:
			if gs.PlayerCount <= 0 {
				if gs.EmptySince.IsZero() {
					gs.EmptySince = now
					gs.LastBeat = now
					_ = m.store.Save(gs)
					continue
				}
				if now.Sub(gs.EmptySince) >= m.cfg.IdleTimeout {
					gs.Status = StatusDraining
					gs.DrainingAt = now
					gs.LastBeat = now
					_ = m.store.Save(gs)
				}
				continue
			}
			gs.EmptySince = time.Time{}
			gs.LastBeat = now
			_ = m.store.Save(gs)
		}
	}
	if remoteOK {
		for id, srv := range remote {
			if _, ok := known[id]; ok {
				continue
			}
			if srv.Alive() && (srv.StartedAt.IsZero() || now.Sub(srv.StartedAt) < orphanGrace) {
				continue
			}
			log.Printf("GSM stopping orphan agent server %s (status %s)", id, srv.Status)
			if err := m.stopRemote(id); err != nil {
				log.Printf("GSM orphan stop %s failed: %v", id, err)
			}
		}
	}
	return reaped, nil
}

func (m *Manager) Start() {
	interval := m.cfg.ReapInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		if reaped, err := m.ReapOnce(); err != nil {
			log.Printf("GSM startup sweep error: %v", err)
		} else if len(reaped) > 0 {
			log.Printf("GSM startup sweep reaped %d gameserver(s): %v", len(reaped), reaped)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				if reaped, err := m.ReapOnce(); err != nil {
					log.Printf("GSM reaper error: %v", err)
				} else if len(reaped) > 0 {
					log.Printf("GSM reaped %d gameserver(s): %v", len(reaped), reaped)
				}
			}
		}
	}()
}

func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.wg.Wait()
}

// Shutdown stops the reaper and waits for it, bounded by ctx. In local mode
// it also stops the gameserver children this process launched and removes
// their records. In remote mode gameservers keep running on the agent host
// so players are not disconnected by a platform deploy.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.stopOnce.Do(func() { close(m.stopCh) })
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if m.agent != nil {
		log.Printf("GSM shutdown: leaving remote gameservers running on the agent")
		return nil
	}
	stopper, ok := m.launcher.(childStopper)
	if !ok || m.store == nil {
		return nil
	}
	grace := m.cfg.ShutdownGrace
	if dl, ok := ctx.Deadline(); ok {
		if left := time.Until(dl) - 500*time.Millisecond; left < grace {
			grace = left
		}
	}
	if grace < 0 {
		grace = 0
	}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		servers, _ := m.store.ListAll()
		var wg sync.WaitGroup
		for _, gs := range servers {
			if !stopper.Owns(gs.PID) {
				continue
			}
			wg.Add(1)
			go func(gs GameServer) {
				defer wg.Done()
				_ = stopper.Stop(gs.PID, grace)
				_ = m.store.Delete(gs.ID)
				_ = m.store.ReleasePort(gs.Port)
			}(gs)
		}
		wg.Wait()
		stopper.StopAll(grace)
	}()
	select {
	case <-stopped:
		log.Printf("GSM shutdown: local gameservers stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// AgentHealth reports gameserver host health for the admin endpoint: the
// agent's /v1/health in remote mode, the binary and log directory locally.
func (m *Manager) AgentHealth(ctx context.Context) (map[string]any, error) {
	if m == nil {
		return nil, ErrUnavailable
	}
	if m.agent != nil {
		out := map[string]any{"mode": "remote", "agent_url": m.cfg.AgentURL, "ok": false}
		h, err := m.agent.Health(ctx)
		if err != nil {
			out["error"] = err.Error()
			return out, err
		}
		out["ok"] = h.OK
		out["servers"] = h.Servers
		out["capacity"] = h.Capacity
		out["version"] = h.Version
		if h.AgentID != "" {
			out["agent_id"] = h.AgentID
		}
		return out, nil
	}
	bin := map[string]any{"path": m.cfg.ServerBin, "exists": false, "size": int64(0)}
	if st, err := os.Stat(m.cfg.ServerBin); err == nil && !st.IsDir() {
		bin["exists"] = true
		bin["size"] = st.Size()
	} else if err != nil {
		bin["error"] = err.Error()
	}
	logs := map[string]any{"dir": m.cfg.LogsDir, "writable": false}
	if err := checkWritable(m.cfg.LogsDir); err == nil {
		logs["writable"] = true
	} else {
		logs["error"] = err.Error()
	}
	ok := bin["exists"] == true && logs["writable"] == true
	return map[string]any{"mode": "local", "ok": ok, "binary": bin, "logs": logs}, nil
}

func checkWritable(dir string) error {
	if dir == "" {
		return errors.New("logs directory is not configured")
	}
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".gsm-write-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(filepath.Clean(name))
}
