package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/service/gsm"
)

const testAgentSecret = "agent-test-secret-0123456789abcdefgh"

// readyScript records its environment, prints the readiness marker and
// idles until signalled.
const readyScript = `#!/bin/sh
dir=$(dirname "$0")
env > "$dir/env-$VERTIGO_SERVER_ID"
echo "$$" >> "$dir/starts-$VERTIGO_SERVER_ID"
echo "booting $*"
echo "Start trigger dispatched"
while :; do sleep 1; done
`

const crashScript = `#!/bin/sh
echo "panic: map failed to load"
exit 3
`

func requireLinux(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("process supervision tests need Linux /proc")
	}
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh not available")
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-server.sh")
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func testConfig(t *testing.T, bin string, portMin, portMax int) agentConfig {
	t.Helper()
	return agentConfig{
		Secret:            testAgentSecret,
		AgentID:           "test-agent",
		PortMin:           portMin,
		PortMax:           portMax,
		ServerBin:         bin,
		ServerMap:         "assets/maps/default.vrtx",
		LogsDir:           t.TempDir(),
		BindHost:          "0.0.0.0",
		ReadyTimeout:      5 * time.Second,
		ShutdownGrace:     time.Second,
		MaxRestarts:       2,
		RestartWindow:     time.Minute,
		APIKey:            "gameserver-api-key",
		APIDomain:         "https://platform.example",
		NetcodeKeyHex:     strings.Repeat("ab", 32),
		NetcodeProtocolID: "42",
		Path:              os.Getenv("PATH"),
		Home:              t.TempDir(),
	}
}

func newTestAgent(t *testing.T, cfg agentConfig, store recordStore) *Agent {
	t.Helper()
	a := newAgent(cfg, store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	a.restartBackoff = 10 * time.Millisecond
	a.pollInterval = 20 * time.Millisecond
	t.Cleanup(func() {
		a.Close(3 * time.Second)
		a.launcher.StopAll(time.Second)
		for _, s := range a.List() {
			if s.PID > 0 && pidMatches(s.PID, s.ID) {
				_ = syscall.Kill(-s.PID, syscall.SIGKILL)
			}
		}
	})
	return a
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func createReq(id string) gsmagent.CreateServerRequest {
	return gsmagent.CreateServerRequest{ServerID: id, GameID: 5, MaxPlayers: 16}
}

func TestSupervisorRestartsThenFails(t *testing.T) {
	requireLinux(t)
	t.Setenv("GSM_AGENT_SECRET", "must-not-leak-to-children")
	script := writeScript(t, readyScript)
	cfg := testConfig(t, script, 47000, 47002)
	store := newMemRecordStore()
	a := newTestAgent(t, cfg, store)

	srv, err := a.Create(createReq("restart1"))
	if err != nil {
		t.Fatal(err)
	}
	if srv.Status != gsmagent.StatusRunning || srv.Port != 47000 || srv.PID <= 0 {
		t.Fatalf("unexpected create result: %+v", srv)
	}
	env, err := os.ReadFile(filepath.Join(filepath.Dir(script), "env-restart1"))
	if err != nil {
		t.Fatal(err)
	}
	envStr := string(env)
	for _, want := range []string{"VERTIGO_APP=server", "VERTIGO_GAME_ID=5", "VERTIGO_SERVER_ID=restart1", "VERTIGO_API_DOMAIN=https://platform.example", "GAMESERVER_API_KEY=gameserver-api-key", "NETCODE_PROTOCOL_ID=42", "NETCODE_PRIVATE_KEY=" + strings.Repeat("ab", 32)} {
		if !strings.Contains(envStr, want) {
			t.Fatalf("child env missing %q:\n%s", want, envStr)
		}
	}
	if strings.Contains(envStr, "GSM_AGENT_SECRET") || strings.Contains(envStr, "REDIS") {
		t.Fatalf("child env leaks agent secrets:\n%s", envStr)
	}

	pid := srv.PID
	for round := 1; round <= cfg.MaxRestarts; round++ {
		if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
			t.Fatal(err)
		}
		waitFor(t, "restart", func() bool {
			cur, ok := a.Get("restart1")
			return ok && cur.Status == gsmagent.StatusRunning && cur.PID > 0 && cur.PID != pid && cur.Restarts == round
		})
		cur, _ := a.Get("restart1")
		if cur.Port != 47000 {
			t.Fatalf("restart must keep the port, got %d", cur.Port)
		}
		pid = cur.PID
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "failed status", func() bool {
		cur, ok := a.Get("restart1")
		return ok && cur.Status == gsmagent.StatusFailed
	})
	a.mu.Lock()
	free := a.ports.Free()
	a.mu.Unlock()
	if free != 3 {
		t.Fatalf("failed server must release its port, free=%d", free)
	}
	if rec, ok := store.get("restart1"); !ok || rec.Status != gsmagent.StatusFailed {
		t.Fatalf("failed status must be persisted: %+v", rec)
	}
	if h := a.Health(); h.Servers != 0 || h.Capacity != 3 {
		t.Fatalf("unexpected health: %+v", h)
	}
	if err := a.Stop("restart1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.Get("restart1"); ok {
		t.Fatal("stopped failed server must be removed")
	}
}

func TestStopTerminatesProcessGroup(t *testing.T) {
	requireLinux(t)
	script := writeScript(t, readyScript)
	store := newMemRecordStore()
	a := newTestAgent(t, testConfig(t, script, 47100, 47100), store)
	srv, err := a.Create(createReq("stopme01"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Stop("stopme01"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "process exit", func() bool { return !pidMatches(srv.PID, "stopme01") })
	if _, ok := store.get("stopme01"); ok {
		t.Fatal("record must be deleted after stop")
	}
	if _, err := a.Create(createReq("stopme02")); err != nil {
		t.Fatalf("port should be free again: %v", err)
	}
	if err := a.Stop("nothere1"); !errors.Is(err, errNotFound) {
		t.Fatalf("expected errNotFound, got %v", err)
	}
}

func TestReconcileAdoptsRestartsAndKeepsFailed(t *testing.T) {
	requireLinux(t)
	script := writeScript(t, readyScript)
	cfg := testConfig(t, script, 47200, 47202)

	// A gameserver left behind by a previous agent run.
	prev := gsm.NewProcessLauncherWithOptions(gsm.ProcessOptions{Bin: script, LogsDir: cfg.LogsDir, Env: cfg.childEnv(), Setpgid: true, APIDomain: cfg.APIDomain})
	oldPID, err := prev.LaunchMap("adopt001", 5, 47200, cfg.ServerMap)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { prev.StopAll(time.Second) })
	if err := prev.WaitHealthy("adopt001", 47200, oldPID, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	store := newMemRecordStore()
	now := time.Now()
	ctx := context.Background()
	_ = store.Save(ctx, record{ID: "adopt001", GameID: 5, MapPath: cfg.ServerMap, MaxPlayers: 8, Port: 47200, PID: oldPID, Status: gsmagent.StatusRunning, CreatedAt: now})
	_ = store.Save(ctx, record{ID: "dead0001", GameID: 6, MapPath: cfg.ServerMap, MaxPlayers: 8, Port: 47201, PID: 0, Status: gsmagent.StatusRunning, CreatedAt: now})
	_ = store.Save(ctx, record{ID: "fail0001", GameID: 7, MapPath: cfg.ServerMap, MaxPlayers: 8, Port: 47202, Status: gsmagent.StatusFailed, FailedAt: now, CreatedAt: now})

	a := newTestAgent(t, cfg, store)
	if err := a.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	adopted, ok := a.Get("adopt001")
	if !ok || adopted.PID != oldPID || adopted.Status != gsmagent.StatusRunning || adopted.Restarts != 0 {
		t.Fatalf("expected adoption of pid %d, got %+v", oldPID, adopted)
	}
	waitFor(t, "dead server restart", func() bool {
		cur, ok := a.Get("dead0001")
		return ok && cur.Status == gsmagent.StatusRunning && cur.PID > 0 && cur.Restarts == 1
	})
	if failed, ok := a.Get("fail0001"); !ok || failed.Status != gsmagent.StatusFailed {
		t.Fatalf("failed record should be kept: %+v", failed)
	}
	a.mu.Lock()
	free := a.ports.Free()
	a.mu.Unlock()
	if free != 1 {
		t.Fatalf("failed server port must stay released, free=%d", free)
	}

	// The adopted process is not our child: exit is detected by polling.
	if err := syscall.Kill(-oldPID, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "adopted server restart", func() bool {
		cur, ok := a.Get("adopt001")
		return ok && cur.Status == gsmagent.StatusRunning && cur.PID != oldPID && cur.Restarts == 1
	})
}

func TestHTTPProtocol(t *testing.T) {
	requireLinux(t)
	script := writeScript(t, readyScript)
	a := newTestAgent(t, testConfig(t, script, 47300, 47300), newMemRecordStore())
	ts := httptest.NewServer(a.routes(gsmagent.NewMemoryNonceStore()))
	defer ts.Close()
	c, err := gsmagent.NewClient(ts.URL, []byte(testAgentSecret), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	var apiErr *gsmagent.APIError
	if _, err := c.CreateServer(ctx, gsmagent.CreateServerRequest{ServerID: "httpsrv1", GameID: 1, MaxPlayers: 0}); !errors.As(err, &apiErr) || apiErr.Status != 400 {
		t.Fatalf("expected 400 for bad max_players, got %v", err)
	}
	srv, err := c.CreateServer(ctx, gsmagent.CreateServerRequest{ServerID: "httpsrv1", GameID: 1, MaxPlayers: 10})
	if err != nil {
		t.Fatal(err)
	}
	if srv.Port != 47300 || srv.Status != gsmagent.StatusRunning || srv.GameID != 1 || srv.MaxPlayers != 10 || srv.StartedAt.IsZero() {
		t.Fatalf("unexpected server: %+v", srv)
	}
	existing, err := c.CreateServer(ctx, gsmagent.CreateServerRequest{ServerID: "httpsrv1", GameID: 1, MaxPlayers: 10})
	if !errors.Is(err, gsmagent.ErrExists) || existing.ID != "httpsrv1" || existing.Port != 47300 {
		t.Fatalf("expected existing record with ErrExists, got %+v %v", existing, err)
	}
	if _, err := c.CreateServer(ctx, gsmagent.CreateServerRequest{ServerID: "httpsrv2", GameID: 1, MaxPlayers: 10}); !errors.Is(err, gsmagent.ErrCapacity) {
		t.Fatalf("expected ErrCapacity, got %v", err)
	}
	if got, err := c.GetServer(ctx, "httpsrv1"); err != nil || got.PID != srv.PID {
		t.Fatalf("get: %+v %v", got, err)
	}
	if list, err := c.ListServers(ctx); err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	if h, err := c.Health(ctx); err != nil || !h.OK || h.Servers != 1 || h.Capacity != 0 || h.AgentID != "test-agent" {
		t.Fatalf("health: %+v %v", h, err)
	}
	if _, err := c.GetServer(ctx, "bad!"); !errors.As(err, &apiErr) || apiErr.Status != 400 {
		t.Fatalf("expected 400 for invalid id, got %v", err)
	}
	if err := c.StopServer(ctx, "httpsrv1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetServer(ctx, "httpsrv1"); !errors.Is(err, gsmagent.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after stop, got %v", err)
	}
	if err := c.StopServer(ctx, "httpsrv1"); !errors.Is(err, gsmagent.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for second stop, got %v", err)
	}

	bad, _ := gsmagent.NewClient(ts.URL, []byte("wrong-secret-wrong-secret-wrong-secret"), time.Second)
	if _, err := bad.ListServers(ctx); !errors.Is(err, gsmagent.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestHTTPNotReady(t *testing.T) {
	requireLinux(t)
	script := writeScript(t, crashScript)
	a := newTestAgent(t, testConfig(t, script, 47400, 47400), newMemRecordStore())
	ts := httptest.NewServer(a.routes(gsmagent.NewMemoryNonceStore()))
	defer ts.Close()
	c, _ := gsmagent.NewClient(ts.URL, []byte(testAgentSecret), 5*time.Second)
	_, err := c.CreateServer(context.Background(), createReq("crashes1"))
	if !errors.Is(err, gsmagent.ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
	if !strings.Contains(err.Error(), "map failed to load") {
		t.Fatalf("expected log tail in error, got %v", err)
	}
	if _, ok := a.Get("crashes1"); ok {
		t.Fatal("failed launch must not leave a record")
	}
	a.mu.Lock()
	free := a.ports.Free()
	a.mu.Unlock()
	if free != 1 {
		t.Fatalf("failed launch must release the port, free=%d", free)
	}
}

func TestMaxServers(t *testing.T) {
	requireLinux(t)
	script := writeScript(t, readyScript)
	cfg := testConfig(t, script, 47500, 47505)
	cfg.MaxServers = 1
	a := newTestAgent(t, cfg, newMemRecordStore())
	if _, err := a.Create(createReq("maxsrv01")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Create(createReq("maxsrv02")); !errors.Is(err, errCapacity) {
		t.Fatalf("expected capacity error, got %v", err)
	}
	if h := a.Health(); h.Capacity != 0 {
		t.Fatalf("capacity must respect GSM_MAX_SERVERS, got %d", h.Capacity)
	}
}
