//go:build !windows

package gsm

import (
	"os"
	"os/exec"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

func signalProc(pid int, proc *os.Process, group bool, sig syscall.Signal) error {
	if group && pid > 0 {
		if err := syscall.Kill(-pid, sig); err == nil {
			return nil
		}
	}
	if proc != nil {
		return proc.Signal(sig)
	}
	if pid > 0 {
		return syscall.Kill(pid, sig)
	}
	return nil
}

func signalTerm(pid int, proc *os.Process, group bool) error {
	return signalProc(pid, proc, group, syscall.SIGTERM)
}

func signalKill(pid int, proc *os.Process, group bool) error {
	return signalProc(pid, proc, group, syscall.SIGKILL)
}
