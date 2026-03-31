//go:build !darwin && !linux

package updater

import (
	"fmt"
	"os/exec"
	"time"
)

func execCommand(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

func detachCommand(_ *exec.Cmd) {}

func terminatePID(pid int) error {
	return fmt.Errorf("terminate pid %d not supported on this platform", pid)
}

func waitForExit(pid int, timeout time.Duration) error {
	return fmt.Errorf("wait for pid %d exit not supported on this platform (%s)", pid, timeout)
}
