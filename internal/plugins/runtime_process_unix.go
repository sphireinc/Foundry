//go:build darwin || linux

package plugins

import (
	"os"
	"os/exec"
	"syscall"
)

func isolateRPCProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Killing the process group also stops a source runner's plugin child. Killing
// just `go run` would leave its actual RPC server alive with inherited pipes.
func killRPCProcess(process *os.Process) {
	_ = syscall.Kill(-process.Pid, syscall.SIGKILL)
}
