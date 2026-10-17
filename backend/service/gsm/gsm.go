package gsm

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
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
	launchWait   = 20 * time.Second
	joinAttempts = 3
)

type Manager struct {
	cfg      Config
	store    Store
	launcher Launcher
	resolve  GameResolver

	spawnMu sync.Mutex
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

func NewManager(cfg Config, store Store, launcher Launcher, resolve GameResolver) *Manager {
	if store == nil {
		store = NewMemoryStore()
	}
	return &Manager{
		cfg:      cfg,
		store:    store,
		launcher: launcher,
		resolve:  resolve,
		stopCh:   make(chan struct{}),
	}
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

func (m *Manager) Liveness() (map[string]bool, error) {
	if m == nil || m.store == nil || m.launcher == nil {
		return nil, ErrUnavailable
	}
	servers, err := m.store.ListAll()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(servers))
	for _, gs := range servers {
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
	if m == nil || m.store == nil || m.launcher == nil {
		return GameServer{}, false, ErrUnavailable
	}
	m.spawnMu.Lock()
	defer m.spawnMu.Unlock()
	info, err := m.resolve(gameID)
	if err != nil {
		return GameServer{}, false, ErrGameMissing
	}
	if info.Private || info.Locked {
		return GameServer{}, false, ErrGameClosed
	}
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

func (m *Manager) spawn(gameID, max int, member string) (GameServer, error) {
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
	if err := m.store.Save(gs); err != nil {
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
	if err := m.launcher.WaitHealthy(id, port, pid, launchWait); err != nil {
		_ = m.launcher.Kill(pid, port)
		_ = m.store.Delete(id)
		_ = m.store.ReleasePort(port)
		log.Printf("GSM spawn %s pid %d port %d never became healthy: %v", id, pid, port, err)
		return GameServer{}, fmt.Errorf("%w: server did not start: %v", ErrLaunch, err)
	}
	count, joined, _, err := m.store.TryJoin(id, member, max)
	if err != nil || !joined {
		_ = m.launcher.Kill(pid, port)
		_ = m.store.Delete(id)
		_ = m.store.ReleasePort(port)
		if err != nil {
			return GameServer{}, err
		}
		log.Printf("GSM spawn %s lost its join race right after launch", id)
		return GameServer{}, fmt.Errorf("%w: join race lost", ErrLaunch)
	}
	gs.Status = StatusActive
	gs.PlayerCount = count
	gs.LastBeat = time.Now()
	_ = m.store.Save(gs)
	gs.Host = m.cfg.PublicHost
	log.Printf("GSM spawn: server %s for game %d active on port %d pid %d", id, gameID, port, pid)
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
	if m == nil || m.store == nil || m.launcher == nil {
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
	if info.Private || info.Locked {
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
	if m == nil || m.store == nil || m.launcher == nil {
		return ErrUnavailable
	}
	gs, found, err := m.store.Get(id)
	if err != nil {
		return err
	}
	if !found {
		return ErrMissing
	}
	_ = m.launcher.Kill(gs.PID, gs.Port)
	_ = m.store.Delete(id)
	_ = m.store.ReleasePort(gs.Port)
	return nil
}

func (m *Manager) cleanup(gs GameServer) {
	_ = m.launcher.Kill(gs.PID, gs.Port)
	_ = m.store.Delete(gs.ID)
	_ = m.store.ReleasePort(gs.Port)
}

func (m *Manager) ReapOnce() ([]string, error) {
	if m == nil || m.store == nil || m.launcher == nil {
		return nil, ErrUnavailable
	}
	servers, err := m.store.ListAll()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var reaped []string
	for _, gs := range servers {
		alive := m.launcher.Alive(gs.PID, gs.Port)
		if !alive {
			if gs.Status == StatusStarting && now.Sub(gs.CreatedAt) < startGrace {
				continue
			}
			m.cleanup(gs)
			reaped = append(reaped, gs.ID)
			continue
		}
		switch gs.Status {
		case StatusStarting:
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
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
	m.wg.Wait()
}
