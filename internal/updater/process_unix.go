//go:build darwin || linux

package updater

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func detachCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func terminatePID(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func waitForExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err != nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}
