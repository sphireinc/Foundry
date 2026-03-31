//go:build darwin || linux

package standalone

import (
	"os/exec"
	"syscall"
)

func detachProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

func sendTerminate(pid int) error {
	return syscall.Kill(pid, syscall.SIGTERM)
}

func sendKill(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}
