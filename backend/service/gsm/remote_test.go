package gsm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"vertexia-frontend/backend/gsmagent"
)

var remoteSecret = "remote-test-secret-0123456789abcdef"

type fakeAgent struct {
	mu        sync.Mutex
	servers   map[string]gsmagent.Server
	nextPort  int
	failWith  int
	creates   int
	lists     int
	deletes   []string
	createReq []gsmagent.CreateServerRequest
}

func newFakeAgent(t *testing.T) (*fakeAgent, *gsmagent.Client) {
	t.Helper()
	fa := &fakeAgent{servers: make(map[string]gsmagent.Server), nextPort: 7000}
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	mux.HandleFunc("POST /v1/servers", func(w http.ResponseWriter, r *http.Request) {
		var req gsmagent.CreateServerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, gsmagent.ErrorResponse{Error: "bad json"})
			return
		}
		fa.mu.Lock()
		defer fa.mu.Unlock()
		fa.creates++
		fa.createReq = append(fa.createReq, req)
		switch fa.failWith {
		case http.StatusConflict:
			writeJSON(w, http.StatusConflict, gsmagent.ErrorResponse{Error: gsmagent.ErrorCapacity})
			return
		case http.StatusServiceUnavailable:
			writeJSON(w, http.StatusServiceUnavailable, gsmagent.ErrorResponse{Error: "gameserver exited during startup"})
			return
		}
		fa.nextPort++
		srv := gsmagent.Server{ID: req.ServerID, Port: fa.nextPort, PID: 1000 + fa.nextPort, Status: gsmagent.StatusRunning, GameID: req.GameID, MaxPlayers: req.MaxPlayers, StartedAt: time.Now()}
		fa.servers[req.ServerID] = srv
		writeJSON(w, http.StatusCreated, srv)
	})
	mux.HandleFunc("GET /v1/servers", func(w http.ResponseWriter, r *http.Request) {
		fa.mu.Lock()
		defer fa.mu.Unlock()
		fa.lists++
		out := gsmagent.ListServersResponse{Servers: []gsmagent.Server{}}
		for _, s := range fa.servers {
			out.Servers = append(out.Servers, s)
		}
		writeJSON(w, http.StatusOK, out)
	})
	mux.HandleFunc("DELETE /v1/servers/{id}", func(w http.ResponseWriter, r *http.Request) {
		fa.mu.Lock()
		defer fa.mu.Unlock()
		id := r.PathValue("id")
		fa.deletes = append(fa.deletes, id)
		if _, ok := fa.servers[id]; !ok {
			writeJSON(w, http.StatusNotFound, gsmagent.ErrorResponse{Error: "not found"})
			return
		}
		delete(fa.servers, id)
		writeJSON(w, http.StatusOK, gsmagent.StopServerResponse{Stopped: true})
	})
	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		fa.mu.Lock()
		defer fa.mu.Unlock()
		writeJSON(w, http.StatusOK, gsmagent.Health{OK: true, Servers: len(fa.servers), Capacity: 10, Version: "fake"})
	})
	srv := httptest.NewServer(gsmagent.Middleware([]byte(remoteSecret), gsmagent.NewMemoryNonceStore(), nil, mux))
	t.Cleanup(srv.Close)
	client, err := gsmagent.NewClient(srv.URL, []byte(remoteSecret), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return fa, client
}

func (fa *fakeAgent) snapshot() (creates, lists int, deletes []string) {
	fa.mu.Lock()
	defer fa.mu.Unlock()
	return fa.creates, fa.lists, append([]string(nil), fa.deletes...)
}

func (fa *fakeAgent) remove(id string) {
	fa.mu.Lock()
	defer fa.mu.Unlock()
	delete(fa.servers, id)
}

func remoteManager(t *testing.T, maxPlayers int) (*Manager, *fakeAgent) {
	t.Helper()
	fa, client := newFakeAgent(t)
	cfg := DefaultConfig()
	cfg.PortMin = 5100
	cfg.PortMax = 5105
	cfg.IdleTimeout = 50 * time.Millisecond
	cfg.ShutdownGrace = 10 * time.Millisecond
	cfg.ReadyTimeout = 2 * time.Second
	cfg.AgentTimeout = 2 * time.Second
	cfg.AgentURL = client.BaseURL()
	m := NewRemoteManager(cfg, NewMemoryStore(), client, func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: maxPlayers}, nil
	})
	return m, fa
}

func TestRemoteJoinSpawnsViaAgent(t *testing.T) {
	m, fa := remoteManager(t, 100)
	if !m.Remote() {
		t.Fatal("expected remote mode")
	}
	first, redirected, err := m.Join(7, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if redirected {
		t.Fatal("first join should spawn")
	}
	if first.Port != 7001 {
		t.Fatalf("expected the agent-allocated port 7001, got %d", first.Port)
	}
	stored, err := m.Get(first.ID)
	if err != nil || stored.Port != 7001 || stored.Status != StatusActive {
		t.Fatalf("stored record: %+v err=%v", stored, err)
	}
	creates, _, _ := fa.snapshot()
	if creates != 1 {
		t.Fatalf("expected 1 create call, got %d", creates)
	}
	if fa.createReq[0].ServerID != first.ID || fa.createReq[0].GameID != 7 || fa.createReq[0].MaxPlayers != 100 {
		t.Fatalf("unexpected create request: %+v", fa.createReq[0])
	}
	second, redirected, err := m.Join(7, "bob")
	if err != nil || !redirected || second.ID != first.ID {
		t.Fatalf("second join should redirect: %+v redirected=%v err=%v", second, redirected, err)
	}
	if creates, _, _ := fa.snapshot(); creates != 1 {
		t.Fatalf("redirect must not create, got %d creates", creates)
	}
	for port := m.cfg.PortMin; port <= m.cfg.PortMax; port++ {
		ok, err := m.store.ClaimPort(port)
		if err != nil || !ok {
			t.Fatalf("remote mode must not claim local port %d", port)
		}
	}
	live, err := m.Liveness()
	if err != nil || !live[first.ID] {
		t.Fatalf("liveness: %v err=%v", live, err)
	}
	h, err := m.AgentHealth(context.Background())
	if err != nil || h["mode"] != "remote" || h["ok"] != true {
		t.Fatalf("agent health: %v err=%v", h, err)
	}
}

func TestRemoteCapacityCap(t *testing.T) {
	m, fa := remoteManager(t, 2)
	m.cfg.MaxServers = 1
	if _, _, err := m.Join(9, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(9, "u2"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(9, "u3"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("expected ErrNoCapacity at the cap, got %v", err)
	}
	if _, _, err := m.Join(10, "u4"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("cap must apply across games, got %v", err)
	}
	if creates, _, _ := fa.snapshot(); creates != 1 {
		t.Fatalf("expected no agent call at the cap, got %d creates", creates)
	}
}

func TestRemoteAgentCapacityConflict(t *testing.T) {
	m, fa := remoteManager(t, 100)
	fa.failWith = http.StatusConflict
	if _, _, err := m.Join(11, "alice"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("expected ErrNoCapacity from agent 409, got %v", err)
	}
	if all, _ := m.ListAll(); len(all) != 0 {
		t.Fatalf("failed spawn must not leave records: %+v", all)
	}
}

func TestRemoteAgentNotReady(t *testing.T) {
	m, fa := remoteManager(t, 100)
	fa.failWith = http.StatusServiceUnavailable
	if _, _, err := m.Join(12, "alice"); !errors.Is(err, ErrLaunch) {
		t.Fatalf("expected ErrLaunch from agent 503, got %v", err)
	}
	if all, _ := m.ListAll(); len(all) != 0 {
		t.Fatalf("failed spawn must not leave records: %+v", all)
	}
}

func TestRemoteHeartbeatTimeoutEviction(t *testing.T) {
	m, fa := remoteManager(t, 2)
	m.cfg.HeartbeatTimeout = 60 * time.Millisecond
	stale, _, err := m.Join(13, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(13, "a2"); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := m.Join(13, "a3")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ID == stale.ID {
		t.Fatal("expected a second server")
	}
	if _, err := m.Heartbeat(stale.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := m.Get(stale.ID)
	if got.LastHeartbeat.IsZero() {
		t.Fatal("heartbeat must set LastHeartbeat")
	}
	time.Sleep(90 * time.Millisecond)
	if _, err := m.Heartbeat(fresh.ID); err != nil {
		t.Fatal(err)
	}
	_, listsBefore, _ := fa.snapshot()
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	_, listsAfter, deletes := fa.snapshot()
	if listsAfter-listsBefore != 1 {
		t.Fatalf("expected exactly one ListServers call per sweep, got %d", listsAfter-listsBefore)
	}
	if len(reaped) != 1 || reaped[0] != stale.ID {
		t.Fatalf("expected %s evicted, got %v", stale.ID, reaped)
	}
	if len(deletes) != 1 || deletes[0] != stale.ID {
		t.Fatalf("expected agent stop of %s, got %v", stale.ID, deletes)
	}
	if _, err := m.Get(stale.ID); !errors.Is(err, ErrMissing) {
		t.Fatalf("evicted record should be gone, got %v", err)
	}
	if _, err := m.Get(fresh.ID); err != nil {
		t.Fatalf("heartbeating server must survive: %v", err)
	}
}

func TestRemoteNeverHeartbeatUsesCreatedAt(t *testing.T) {
	m, fa := remoteManager(t, 100)
	m.cfg.HeartbeatTimeout = 40 * time.Millisecond
	gs, _, err := m.Join(14, "alice")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != gs.ID {
		t.Fatalf("silent server should be evicted, got %v", reaped)
	}
	if _, _, deletes := fa.snapshot(); len(deletes) != 1 {
		t.Fatalf("expected agent stop, got %v", deletes)
	}
}

func TestRemoteIdleScaleDownStopsViaAgent(t *testing.T) {
	m, fa := remoteManager(t, 100)
	m.cfg.HeartbeatTimeout = 0
	gs, _, err := m.Join(15, "erin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Leave(gs.ID, "erin"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	if _, err := m.ReapOnce(); err != nil {
		t.Fatal(err)
	}
	cur, err := m.Get(gs.ID)
	if err != nil || cur.Status != StatusDraining {
		t.Fatalf("expected draining, got %+v err=%v", cur, err)
	}
	time.Sleep(30 * time.Millisecond)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != gs.ID {
		t.Fatalf("expected wind-down of %s, got %v", gs.ID, reaped)
	}
	if _, _, deletes := fa.snapshot(); len(deletes) != 1 || deletes[0] != gs.ID {
		t.Fatalf("expected stop through the agent, got %v", deletes)
	}
}

func TestRemoteDeadServerReaped(t *testing.T) {
	m, fa := remoteManager(t, 100)
	gs, _, err := m.Join(16, "frank")
	if err != nil {
		t.Fatal(err)
	}
	fa.remove(gs.ID)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != gs.ID {
		t.Fatalf("expected dead server reaped, got %v", reaped)
	}
}

func TestRemoteOrphanSweep(t *testing.T) {
	m, fa := remoteManager(t, 100)
	fa.mu.Lock()
	fa.servers["orphan01"] = gsmagent.Server{ID: "orphan01", Port: 7999, Status: gsmagent.StatusRunning, GameID: 1, StartedAt: time.Now().Add(-2 * orphanGrace)}
	fa.servers["young001"] = gsmagent.Server{ID: "young001", Port: 7998, Status: gsmagent.StatusRunning, GameID: 1, StartedAt: time.Now()}
	fa.mu.Unlock()
	if _, err := m.ReapOnce(); err != nil {
		t.Fatal(err)
	}
	_, _, deletes := fa.snapshot()
	if len(deletes) != 1 || deletes[0] != "orphan01" {
		t.Fatalf("expected only the old orphan stopped, got %v", deletes)
	}
}

func TestRemoteShutdownLeavesServersRunning(t *testing.T) {
	m, fa := remoteManager(t, 100)
	m.Start()
	if _, _, err := m.Join(17, "gail"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, deletes := fa.snapshot(); len(deletes) != 0 {
		t.Fatalf("remote shutdown must not stop servers, got %v", deletes)
	}
}

func TestLocalHeartbeatTimeoutAndMaxServers(t *testing.T) {
	m, launcher := testManager()
	m.cfg.MaxServers = 1
	m.cfg.HeartbeatTimeout = 40 * time.Millisecond
	gs, _, err := m.Join(61, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(62, "bob"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("expected ErrNoCapacity, got %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != gs.ID {
		t.Fatalf("expected heartbeat eviction, got %v", reaped)
	}
	launcher.mu.Lock()
	killed := len(launcher.killed)
	launcher.mu.Unlock()
	if killed != 1 {
		t.Fatalf("expected the process to be killed, got %d kills", killed)
	}
	if ok, _ := m.store.ClaimPort(gs.Port); !ok {
		t.Fatal("evicted server port must be released")
	}
}

func TestMemLockerSerializes(t *testing.T) {
	l := newMemLocker()
	unlock, err := l.Lock(context.Background(), "k", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := l.Lock(ctx, "k", time.Second); err == nil {
		t.Fatal("second lock must wait for the first")
	}
	if u2, err := l.Lock(context.Background(), "other", time.Second); err != nil {
		t.Fatal(err)
	} else {
		u2()
	}
	unlock()
	unlock()
	u3, err := l.Lock(context.Background(), "k", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	u3()
}

func TestNewRedisStoreNil(t *testing.T) {
	if NewRedisStore(nil) != nil {
		t.Fatal("nil client must not produce a store")
	}
}

func TestRemoteStuckStartingRecordCleaned(t *testing.T) {
	m, fa := remoteManager(t, 100)
	created := time.Now().Add(-startGrace - time.Second)
	fa.mu.Lock()
	fa.servers["stuck001"] = gsmagent.Server{ID: "stuck001", Port: 7500, Status: gsmagent.StatusRunning, GameID: 3, StartedAt: created}
	fa.mu.Unlock()
	if err := m.store.Save(GameServer{ID: "stuck001", GameID: 3, Status: StatusStarting, MaxPlayers: 10, CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != "stuck001" {
		t.Fatalf("expected portless record past start grace reaped, got %v", reaped)
	}
	if _, _, deletes := fa.snapshot(); len(deletes) != 1 || deletes[0] != "stuck001" {
		t.Fatalf("expected agent stop, got %v", deletes)
	}
}
