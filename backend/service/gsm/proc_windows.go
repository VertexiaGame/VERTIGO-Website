//go:build windows

package gsm

import (
	"os"
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {}

func signalTerm(pid int, proc *os.Process, group bool) error {
	return signalKill(pid, proc, group)
}

func signalKill(pid int, proc *os.Process, group bool) error {
	if proc == nil {
		return nil
	}
	return proc.Kill()
}
