//go:build !darwin && !linux

package plugins

import (
	"os"
	"os/exec"
)

func isolateRPCProcess(*exec.Cmd) {}

func killRPCProcess(process *os.Process) {
	_ = process.Kill()
}
