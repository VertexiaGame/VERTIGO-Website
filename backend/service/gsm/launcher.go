package gsm

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const readyMarker = "Start trigger dispatched"

type Launcher interface {
	Launch(id string, gameID int, port int) (int, error)
	WaitHealthy(id string, port int, pid int, timeout time.Duration) error
	Alive(pid int, port int) bool
	Kill(pid int, port int) error
}

type childInfo struct {
	proc *os.Process
	done chan struct{}
}

type ProcessLauncher struct {
	bind    string
	bin     string
	workdir string
	mapPath string
	logsDir string
	apiKey  string

	mu    sync.Mutex
	procs map[int]*childInfo
}

func NewProcessLauncher(bind, bin, workdir, mapPath, logsDir, apiKey string) *ProcessLauncher {
	if bind == "" {
		bind = "127.0.0.1"
	}
	return &ProcessLauncher{
		bind:    bind,
		bin:     bin,
		workdir: workdir,
		mapPath: mapPath,
		logsDir: logsDir,
		apiKey:  apiKey,
		procs:   make(map[int]*childInfo),
	}
}

func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
		out, err := cmd.Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), strconv.Itoa(pid))
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}
	return errors.Is(err, syscall.EPERM)
}

func (l *ProcessLauncher) logPath(id string) string {
	return filepath.Join(l.logsDir, "gs-"+id+".log")
}

func (l *ProcessLauncher) logFile(id string) (*os.File, error) {
	if err := os.MkdirAll(l.logsDir, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(l.logPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

func logTail(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return "no log output"
	}
	if len(raw) > 2048 {
		raw = raw[len(raw)-2048:]
	}
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, 5)
	for i := len(lines) - 1; i >= 0 && len(kept) < 5; i-- {
		if trimmed := strings.TrimSpace(lines[i]); trimmed != "" {
			kept = append([]string{trimmed}, kept...)
		}
	}
	if len(kept) == 0 {
		return "no log output"
	}
	return strings.Join(kept, " | ")
}

func logReady(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), readyMarker)
}

func localAPIDomain() string {
	for _, key := range []string{"PORT", "ADDR", "LISTEN_ADDR"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			return normalizeAPIDomain(raw)
		}
	}
	return "http://127.0.0.1:3000"
}

func normalizeAPIDomain(raw string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(raw), "/")
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, ":") {
		trimmed = "127.0.0.1" + trimmed
	}
	if host, port, ok := strings.Cut(trimmed, ":"); ok && host != "" && port != "" {
		if host == "0.0.0.0" {
			host = "127.0.0.1"
		}
		return "http://" + host + ":" + port
	}
	return "http://127.0.0.1:3000"
}

func (l *ProcessLauncher) Launch(id string, gameID int, port int) (int, error) {
	if strings.TrimSpace(l.bin) == "" {
		return 0, fmt.Errorf("gameserver binary is not configured")
	}
	if _, err := os.Stat(l.bin); err != nil {
		return 0, fmt.Errorf("gameserver binary not found")
	}
	bindAll := l.bind
	if bindAll == "127.0.0.1" || bindAll == "localhost" {
		bindAll = "0.0.0.0"
	}
	args := []string{"--port", strconv.Itoa(port), "--map", l.mapPath, "--bind", bindAll, "--game-id", strconv.Itoa(gameID), "--server-id", id}
	cmd := exec.Command(l.bin, args...)
	if l.workdir != "" {
		cmd.Dir = l.workdir
	}
	logFile, err := l.logFile(id)
	if err != nil {
		return 0, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "VERTIGO_APP=server", "VERTIGO_GAME_ID="+strconv.Itoa(gameID), "VERTIGO_SERVER_ID="+id)
	if strings.TrimSpace(os.Getenv("VERTIGO_API_DOMAIN")) == "" {
		cmd.Env = append(cmd.Env, "VERTIGO_API_DOMAIN="+localAPIDomain())
	}
	if l.apiKey != "" {
		cmd.Env = append(cmd.Env, "GAMESERVER_API_KEY="+l.apiKey)
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, err
	}
	_ = logFile.Close()
	pid := cmd.Process.Pid
	info := &childInfo{proc: cmd.Process, done: make(chan struct{})}
	l.mu.Lock()
	l.procs[pid] = info
	l.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(info.done)
		l.mu.Lock()
		if cur, ok := l.procs[pid]; ok && cur == info {
			delete(l.procs, pid)
		}
		l.mu.Unlock()
	}()
	return pid, nil
}

func (l *ProcessLauncher) child(pid int) *childInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.procs[pid]
}

func (l *ProcessLauncher) running(pid int) bool {
	if info := l.child(pid); info != nil {
		select {
		case <-info.done:
			return false
		default:
			return true
		}
	}
	return processRunning(pid)
}

func (l *ProcessLauncher) WaitHealthy(id string, port int, pid int, timeout time.Duration) error {
	path := l.logPath(id)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !l.running(pid) {
			return fmt.Errorf("gameserver exited during startup: %s", logTail(path))
		}
		if logReady(path) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	if logReady(path) {
		return nil
	}
	if !l.running(pid) {
		return fmt.Errorf("gameserver exited during startup: %s", logTail(path))
	}
	return fmt.Errorf("gameserver did not become healthy in time: %s", logTail(path))
}

func (l *ProcessLauncher) Alive(pid int, port int) bool {
	return l.running(pid)
}

func (l *ProcessLauncher) Kill(pid int, port int) error {
	if info := l.child(pid); info != nil {
		_ = info.proc.Kill()
		l.mu.Lock()
		if cur, ok := l.procs[pid]; ok && cur == info {
			delete(l.procs, pid)
		}
		l.mu.Unlock()
		return nil
	}
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run()
		return nil
	}
	if proc, err := os.FindProcess(pid); err == nil {
		_ = proc.Kill()
	}
	return nil
}
