package gsm

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeLauncher struct {
	mu     sync.Mutex
	next   int
	alive  map[int]bool
	ports  map[int]int
	killed []int
}

func newFakeLauncher() *fakeLauncher {
	return &fakeLauncher{alive: make(map[int]bool), ports: make(map[int]int)}
}

func (f *fakeLauncher) Launch(id string, gameID int, port int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	f.alive[f.next] = true
	f.ports[port] = f.next
	return f.next, nil
}

func (f *fakeLauncher) WaitHealthy(id string, port int, pid int, timeout time.Duration) error {
	return nil
}

func (f *fakeLauncher) Alive(pid int, port int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pid <= 0 {
		owner, ok := f.ports[port]
		return ok && f.alive[owner]
	}
	return f.alive[pid]
}

func (f *fakeLauncher) Kill(pid int, port int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alive[pid] = false
	if owner, ok := f.ports[port]; ok && owner == pid {
		delete(f.ports, port)
	}
	f.killed = append(f.killed, pid)
	return nil
}

func (f *fakeLauncher) setAlive(pid int, alive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alive[pid] = alive
}

func testManager() (*Manager, *fakeLauncher) {
	cfg := DefaultConfig()
	cfg.PortMin = 5100
	cfg.PortMax = 5105
	cfg.IdleTimeout = 50 * time.Millisecond
	cfg.ShutdownGrace = 10 * time.Millisecond
	launcher := newFakeLauncher()
	m := NewManager(cfg, NewMemoryStore(), launcher, func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: 100}, nil
	})
	return m, launcher
}

func TestGenerateIDFormat(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 200; i++ {
		id, err := GenerateID()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidID(id) {
			t.Fatalf("invalid id %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) < 190 {
		t.Fatalf("ids are not random enough: %d unique of 200", len(seen))
	}
}

func TestJoinCreatesThenRedirects(t *testing.T) {
	m, _ := testManager()
	first, redirected, err := m.Join(7, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if redirected {
		t.Fatal("first join should launch, not redirect")
	}
	if !ValidID(first.ID) {
		t.Fatalf("bad id %q", first.ID)
	}
	if first.Port < 5100 || first.Port > 5105 {
		t.Fatalf("port out of range: %d", first.Port)
	}
	if first.PlayerCount != 1 {
		t.Fatalf("expected 1 player, got %d", first.PlayerCount)
	}
	second, redirected, err := m.Join(7, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if !redirected {
		t.Fatal("second join should redirect to existing server")
	}
	if second.ID != first.ID {
		t.Fatalf("expected redirect to %s, got %s", first.ID, second.ID)
	}
	if second.PlayerCount != 2 {
		t.Fatalf("expected 2 players, got %d", second.PlayerCount)
	}
}

func TestFullServerSpawnsNew(t *testing.T) {
	m, _ := testManager()
	m.resolve = func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: 2}, nil
	}
	a, _, err := m.Join(9, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(9, "u2"); err != nil {
		t.Fatal(err)
	}
	b, redirected, err := m.Join(9, "u3")
	if err != nil {
		t.Fatal(err)
	}
	if redirected {
		t.Fatal("full server should not receive a redirect")
	}
	if b.ID == a.ID {
		t.Fatal("expected a new server when existing is full")
	}
}

func TestLeaveMarksEmpty(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(11, "carol")
	if err != nil {
		t.Fatal(err)
	}
	after, err := m.Leave(gs.ID, "carol")
	if err != nil {
		t.Fatal(err)
	}
	if after.PlayerCount != 0 {
		t.Fatalf("expected 0 players, got %d", after.PlayerCount)
	}
	if after.EmptySince.IsZero() {
		t.Fatal("expected EmptySince to be set")
	}
}

func TestReapDeadProcess(t *testing.T) {
	m, launcher := testManager()
	gs, _, err := m.Join(13, "dave")
	if err != nil {
		t.Fatal(err)
	}
	launcher.setAlive(gs.PID, false)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 || reaped[0] != gs.ID {
		t.Fatalf("expected %s reaped, got %v", gs.ID, reaped)
	}
	if _, found, _ := m.store.Get(gs.ID); found {
		t.Fatal("dead server record should be removed")
	}
}

func TestReapIdleWindDown(t *testing.T) {
	m, launcher := testManager()
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
	cur, found, err := m.store.Get(gs.ID)
	if err != nil || !found {
		t.Fatalf("expected draining record, found=%v err=%v", found, err)
	}
	if cur.Status != StatusDraining {
		t.Fatalf("expected draining, got %s", cur.Status)
	}
	time.Sleep(30 * time.Millisecond)
	reaped, err := m.ReapOnce()
	if err != nil {
		t.Fatal(err)
	}
	if len(reaped) != 1 {
		t.Fatalf("expected wind-down kill, got %v", reaped)
	}
	launcher.mu.Lock()
	killed := len(launcher.killed) > 0
	launcher.mu.Unlock()
	if !killed {
		t.Fatal("expected process kill")
	}
}

func TestDrainStopsJoins(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(17, "frank")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Drain(gs.ID); err != nil {
		t.Fatal(err)
	}
	next, redirected, err := m.Join(17, "gail")
	if err != nil {
		t.Fatal(err)
	}
	if redirected || next.ID == gs.ID {
		t.Fatal("draining server must not receive joins")
	}
}

func TestKillFreesPort(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(19, "grace")
	if err != nil {
		t.Fatal(err)
	}
	port := gs.Port
	if err := m.Kill(gs.ID); err != nil {
		t.Fatal(err)
	}
	ok, err := m.store.ClaimPort(port)
	if err != nil || !ok {
		t.Fatalf("port %d should be free after kill", port)
	}
}

func TestLogReadyDetectsMarker(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/gs-test.log"
	if logReady(path) {
		t.Fatal("missing log must not report ready")
	}
	if err := os.WriteFile(path, []byte("Booting\nStill starting\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if logReady(path) {
		t.Fatal("log without marker must not report ready")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("Server entity spawned and Start trigger dispatched\n")
	_ = f.Close()
	if !logReady(path) {
		t.Fatal("log with marker must report ready")
	}
}

func TestLogTailSurfacesRecentLines(t *testing.T) {
	if got := logTail(t.TempDir() + "/missing.log"); got != "no log output" {
		t.Fatalf("expected no-log message, got %q", got)
	}
	dir := t.TempDir()
	path := dir + "/gs-test.log"
	var sb strings.Builder
	for i := 1; i <= 10; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatal(err)
	}
	tail := logTail(path)
	if !strings.Contains(tail, "line 10") || strings.Contains(tail, "line 5") {
		t.Fatalf("expected last 5 lines, got %q", tail)
	}
}

func TestProcessLauncherAliveSelf(t *testing.T) {
	l := NewProcessLauncher("127.0.0.1", "bin", "dir", "map", t.TempDir(), "")
	if l.Alive(0, 0) {
		t.Fatal("pid 0 must not report alive")
	}
	if !l.Alive(os.Getpid(), 0) {
		t.Fatal("current process must report alive")
	}
}

type failingLauncher struct {
	fakeLauncher
	failWith error
}

func (f *failingLauncher) Launch(id string, gameID int, port int) (int, error) {
	return 0, f.failWith
}

func TestLaunchErrorWrapsSentinel(t *testing.T) {
	cfg := DefaultConfig()
	cfg.PortMin = 5200
	cfg.PortMax = 5205
	broken := &failingLauncher{failWith: errors.New("binary missing")}
	m := NewManager(cfg, NewMemoryStore(), broken, func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: 100}, nil
	})
	if _, _, err := m.Join(23, "ivan"); !errors.Is(err, ErrLaunch) {
		t.Fatalf("expected ErrLaunch, got %v", err)
	}
}

func TestNoCapacityWhenRangeFull(t *testing.T) {
	m, _ := testManager()
	m.cfg.PortMin = 5300
	m.cfg.PortMax = 5300
	m.resolve = func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: 2}, nil
	}
	if _, _, err := m.Join(25, "kate"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(25, "leo"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(25, "max"); !errors.Is(err, ErrNoCapacity) {
		t.Fatalf("expected ErrNoCapacity, got %v", err)
	}
}

func TestStorePingAndLiveness(t *testing.T) {
	m, _ := testManager()
	if err := m.Store().Ping(); err != nil {
		t.Fatalf("store ping failed: %v", err)
	}
	gs, _, err := m.Join(27, "mia")
	if err != nil {
		t.Fatal(err)
	}
	live, err := m.Liveness()
	if err != nil {
		t.Fatal(err)
	}
	if !live[gs.ID] {
		t.Fatal("expected fresh server to report alive")
	}
}

func TestReconnectDoesNotInflateCount(t *testing.T) {
	m, _ := testManager()
	first, _, err := m.Join(21, "henry")
	if err != nil {
		t.Fatal(err)
	}
	again, redirected, err := m.Join(21, "henry")
	if err != nil {
		t.Fatal(err)
	}
	if !redirected || again.ID != first.ID {
		t.Fatal("reconnect should redirect to the same server")
	}
	if again.PlayerCount != 1 {
		t.Fatalf("expected 1 player after reconnect, got %d", again.PlayerCount)
	}
}

func TestSyncRosterReconcilesMembers(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(31, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.Join(31, "stale"); err != nil {
		t.Fatal(err)
	}
	synced, err := m.SyncRoster(gs.ID, []string{"bob", "alice", "bob", "", "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if synced.PlayerCount != 2 {
		t.Fatalf("expected 2 players after sync, got %d", synced.PlayerCount)
	}
	members, err := m.Store().Members(gs.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, member := range members {
		got[member] = true
	}
	if !got["alice"] || !got["bob"] || got["stale"] {
		t.Fatalf("unexpected roster after sync: %v", members)
	}
	emptied, err := m.SyncRoster(gs.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if emptied.PlayerCount != 0 {
		t.Fatalf("expected 0 players after empty sync, got %d", emptied.PlayerCount)
	}
}

func TestSyncRosterTruncatesToMax(t *testing.T) {
	m, _ := testManager()
	m.resolve = func(gameID int) (GameInfo, error) {
		return GameInfo{ID: gameID, MaxPlayers: 2}, nil
	}
	gs, _, err := m.Join(33, "alice")
	if err != nil {
		t.Fatal(err)
	}
	synced, err := m.SyncRoster(gs.ID, []string{"alice", "bob", "carol"})
	if err != nil {
		t.Fatal(err)
	}
	if synced.PlayerCount != 2 {
		t.Fatalf("expected roster truncated to 2, got %d", synced.PlayerCount)
	}
	if synced.MaxPlayers != 2 {
		t.Fatalf("expected max 2, got %d", synced.MaxPlayers)
	}
}

func TestSyncRosterMissingServer(t *testing.T) {
	m, _ := testManager()
	if _, err := m.SyncRoster("deadbeef", []string{"alice"}); !errors.Is(err, ErrMissing) {
		t.Fatalf("expected ErrMissing, got %v", err)
	}
}

func confirmedSet(t *testing.T, m *Manager, id string) map[string]bool {
	t.Helper()
	confirmed, err := m.Store().ConfirmedMembers(id)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]bool, len(confirmed))
	for _, member := range confirmed {
		out[member] = true
	}
	return out
}

func TestReservationDoesNotConfirmPresence(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(41, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); len(got) != 0 {
		t.Fatalf("reservation must not confirm presence, got %v", got)
	}
	if _, _, err := m.Join(41, "bob"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.ServerJoin(gs.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	got := confirmedSet(t, m, gs.ID)
	if !got["alice"] || got["bob"] {
		t.Fatalf("only gameserver-joined players must be confirmed, got %v", got)
	}
	if _, _, err := m.ServerJoin(gs.ID, "bob"); err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); !got["alice"] || !got["bob"] {
		t.Fatalf("expected alice and bob confirmed, got %v", got)
	}
}

func TestLeaveUnconfirmsPresence(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(43, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.ServerJoin(gs.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Leave(gs.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); len(got) != 0 {
		t.Fatalf("leave must unconfirm presence, got %v", got)
	}
}

func TestSyncRosterConfirmsPresence(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(45, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.SyncRoster(gs.ID, []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); !got["alice"] || !got["bob"] {
		t.Fatalf("expected roster confirmed, got %v", got)
	}
	if _, err := m.SyncRoster(gs.ID, []string{"bob"}); err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); !got["bob"] || got["alice"] {
		t.Fatalf("expected only bob confirmed, got %v", got)
	}
}

func TestKillClearsPresence(t *testing.T) {
	m, _ := testManager()
	gs, _, err := m.Join(47, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.ServerJoin(gs.ID, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := m.Kill(gs.ID); err != nil {
		t.Fatal(err)
	}
	if got := confirmedSet(t, m, gs.ID); len(got) != 0 {
		t.Fatalf("kill must clear presence, got %v", got)
	}
}
