package gsm

import (
	"errors"
	"fmt"
	"io"
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

// Launcher starts and supervises local gameserver processes.
type Launcher interface {
	Launch(id string, gameID int, port int) (int, error)
	WaitHealthy(id string, port int, pid int, timeout time.Duration) error
	Alive(pid int, port int) bool
	Kill(pid int, port int) error
}

// ProcessOptions configures a ProcessLauncher. It is shared by the
// platform's local development mode and the gsm-agent.
type ProcessOptions struct {
	Bind    string
	Bin     string
	Workdir string
	MapPath string
	LogsDir string
	// APIKey is passed to the child as GAMESERVER_API_KEY when set.
	APIKey string
	// APIDomain is passed as VERTIGO_API_DOMAIN. When empty and the parent
	// environment has none either, a loopback address is derived.
	APIDomain string
	// Env holds extra KEY=VALUE entries for the child. Later entries win.
	Env []string
	// InheritEnv starts the child environment from os.Environ(). The agent
	// sets it to false so its own secrets never reach gameservers.
	InheritEnv bool
	// Setpgid puts each child in its own process group so stops reach the
	// whole group and the child survives a restart of its parent.
	Setpgid bool
}

type childInfo struct {
	proc      *os.Process
	done      chan struct{}
	logOffset int64
}

// ProcessLauncher spawns the gameserver binary as a child process, detects
// readiness from its log and stops it with signals.
type ProcessLauncher struct {
	opts ProcessOptions

	mu    sync.Mutex
	procs map[int]*childInfo
}

// NewProcessLauncher keeps the historic constructor used by local mode: the
// child inherits the platform environment.
func NewProcessLauncher(bind, bin, workdir, mapPath, logsDir, apiKey string) *ProcessLauncher {
	return NewProcessLauncherWithOptions(ProcessOptions{
		Bind:       bind,
		Bin:        bin,
		Workdir:    workdir,
		MapPath:    mapPath,
		LogsDir:    logsDir,
		APIKey:     apiKey,
		InheritEnv: true,
	})
}

func NewProcessLauncherWithOptions(opts ProcessOptions) *ProcessLauncher {
	if opts.Bind == "" {
		opts.Bind = "127.0.0.1"
	}
	opts.Env = append([]string(nil), opts.Env...)
	return &ProcessLauncher{opts: opts, procs: make(map[int]*childInfo)}
}

// Options returns a copy of the launcher options.
func (l *ProcessLauncher) Options() ProcessOptions {
	o := l.opts
	o.Env = append([]string(nil), l.opts.Env...)
	return o
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

// ProcessRunning reports whether pid refers to a live process.
func ProcessRunning(pid int) bool {
	return processRunning(pid)
}

// LogPath returns the log file of server id.
func (l *ProcessLauncher) LogPath(id string) string {
	return filepath.Join(l.opts.LogsDir, "gs-"+id+".log")
}

func (l *ProcessLauncher) logPath(id string) string {
	return l.LogPath(id)
}

func (l *ProcessLauncher) logFile(id string) (*os.File, int64, error) {
	if err := os.MkdirAll(l.opts.LogsDir, 0750); err != nil {
		return nil, 0, err
	}
	f, err := os.OpenFile(l.logPath(id), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return nil, 0, err
	}
	var offset int64
	if st, err := f.Stat(); err == nil {
		offset = st.Size()
	}
	return f, offset, nil
}

// LogTail summarizes the last lines of a log file.
func LogTail(path string) string {
	return logTail(path)
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
	return logReadyFrom(path, 0)
}

// logReadyFrom looks for the readiness marker after offset so a restart on
// an appended log file is not fooled by the previous run's output.
func logReadyFrom(path string, offset int64) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return false
		}
	}
	raw, err := io.ReadAll(io.LimitReader(f, 8<<20))
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

func (l *ProcessLauncher) childEnv(id string, gameID int) []string {
	var env []string
	if l.opts.InheritEnv {
		env = os.Environ()
	}
	env = append(env, l.opts.Env...)
	env = append(env, "VERTIGO_APP=server", "VERTIGO_GAME_ID="+strconv.Itoa(gameID), "VERTIGO_SERVER_ID="+id)
	switch {
	case l.opts.APIDomain != "":
		env = append(env, "VERTIGO_API_DOMAIN="+l.opts.APIDomain)
	case !l.opts.InheritEnv || strings.TrimSpace(os.Getenv("VERTIGO_API_DOMAIN")) == "":
		env = append(env, "VERTIGO_API_DOMAIN="+localAPIDomain())
	}
	if l.opts.APIKey != "" {
		env = append(env, "GAMESERVER_API_KEY="+l.opts.APIKey)
	}
	return env
}

// Launch starts the gameserver with the default map.
func (l *ProcessLauncher) Launch(id string, gameID int, port int) (int, error) {
	return l.LaunchMap(id, gameID, port, l.opts.MapPath)
}

// LaunchMap starts the gameserver with an explicit map path.
func (l *ProcessLauncher) LaunchMap(id string, gameID int, port int, mapPath string) (int, error) {
	if strings.TrimSpace(l.opts.Bin) == "" {
		return 0, fmt.Errorf("gameserver binary is not configured")
	}
	if _, err := os.Stat(l.opts.Bin); err != nil {
		return 0, fmt.Errorf("gameserver binary not found")
	}
	if mapPath == "" {
		mapPath = l.opts.MapPath
	}
	bindAll := l.opts.Bind
	if bindAll == "127.0.0.1" || bindAll == "localhost" {
		bindAll = "0.0.0.0"
	}
	args := []string{"--port", strconv.Itoa(port), "--map", mapPath, "--bind", bindAll, "--game-id", strconv.Itoa(gameID), "--server-id", id}
	cmd := exec.Command(l.opts.Bin, args...)
	if l.opts.Workdir != "" {
		cmd.Dir = l.opts.Workdir
	}
	logFile, offset, err := l.logFile(id)
	if err != nil {
		return 0, err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = l.childEnv(id, gameID)
	if l.opts.Setpgid {
		setProcessGroup(cmd)
	}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, err
	}
	_ = logFile.Close()
	pid := cmd.Process.Pid
	info := &childInfo{proc: cmd.Process, done: make(chan struct{}), logOffset: offset}
	l.mu.Lock()
	l.procs[pid] = info
	l.mu.Unlock()
	go func() {
		_ = cmd.Wait()
		close(info.done)
	}()
	return pid, nil
}

func (l *ProcessLauncher) child(pid int) *childInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.procs[pid]
}

func (l *ProcessLauncher) forget(pid int, info *childInfo) {
	l.mu.Lock()
	if cur, ok := l.procs[pid]; ok && cur == info {
		delete(l.procs, pid)
	}
	l.mu.Unlock()
}

// Owns reports whether pid is a child started by this launcher that has not
// been stopped yet.
func (l *ProcessLauncher) Owns(pid int) bool {
	return l.child(pid) != nil
}

// Exited returns a channel closed when child pid exits, or nil when pid is
// not a child of this launcher (for example a process adopted after an
// agent restart). The child stays tracked until Stop, Kill or Forget.
func (l *ProcessLauncher) Exited(pid int) <-chan struct{} {
	if info := l.child(pid); info != nil {
		return info.done
	}
	return nil
}

// Forget drops bookkeeping for a child that has exited.
func (l *ProcessLauncher) Forget(pid int) {
	if info := l.child(pid); info != nil {
		l.forget(pid, info)
	}
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
	var offset int64
	if info := l.child(pid); info != nil {
		offset = info.logOffset
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !l.running(pid) {
			if logReadyFrom(path, offset) {
				return fmt.Errorf("gameserver exited right after startup: %s", logTail(path))
			}
			return fmt.Errorf("gameserver exited during startup: %s", logTail(path))
		}
		if logReadyFrom(path, offset) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if logReadyFrom(path, offset) && l.running(pid) {
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

// Kill stops pid immediately (SIGKILL, to the whole group with Setpgid).
func (l *ProcessLauncher) Kill(pid int, port int) error {
	if info := l.child(pid); info != nil {
		_ = signalKill(pid, info.proc, l.opts.Setpgid)
		l.forget(pid, info)
		return nil
	}
	if pid <= 0 {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run()
		return nil
	}
	proc, _ := os.FindProcess(pid)
	_ = signalKill(pid, proc, l.opts.Setpgid)
	return nil
}

// Stop asks pid to exit with SIGTERM (to its process group with Setpgid)
// and escalates to SIGKILL once grace has elapsed.
func (l *ProcessLauncher) Stop(pid int, grace time.Duration) error {
	if pid <= 0 {
		return nil
	}
	info := l.child(pid)
	var proc *os.Process
	if info != nil {
		proc = info.proc
	} else {
		proc, _ = os.FindProcess(pid)
	}
	exited := func() bool {
		if info != nil {
			select {
			case <-info.done:
				return true
			default:
				return false
			}
		}
		return !processRunning(pid)
	}
	if !exited() {
		_ = signalTerm(pid, proc, l.opts.Setpgid)
		deadline := time.Now().Add(grace)
		for !exited() && time.Now().Before(deadline) {
			time.Sleep(50 * time.Millisecond)
		}
	}
	// Always signal the group: helpers forked by the gameserver may outlive it.
	_ = signalKill(pid, proc, l.opts.Setpgid)
	if info != nil {
		select {
		case <-info.done:
		case <-time.After(5 * time.Second):
		}
		l.forget(pid, info)
	}
	return nil
}

// StopAll stops every tracked child concurrently and returns the pids.
func (l *ProcessLauncher) StopAll(grace time.Duration) []int {
	l.mu.Lock()
	pids := make([]int, 0, len(l.procs))
	for pid := range l.procs {
		pids = append(pids, pid)
	}
	l.mu.Unlock()
	var wg sync.WaitGroup
	for _, pid := range pids {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			_ = l.Stop(pid, grace)
		}(pid)
	}
	wg.Wait()
	return pids
}
