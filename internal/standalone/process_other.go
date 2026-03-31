//go:build !darwin && !linux

package standalone

import (
	"fmt"
	"os/exec"
)

func detachProcess(cmd *exec.Cmd) {}

func IsProcessAlive(pid int) bool {
	return false
}

func sendTerminate(pid int) error {
	return fmt.Errorf("standalone process control is not supported on this platform")
}

func sendKill(pid int) error {
	return fmt.Errorf("standalone process control is not supported on this platform")
}
