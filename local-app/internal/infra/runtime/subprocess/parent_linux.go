package subprocess

import (
	"os/exec"
	"syscall"
)

// Kernel ownership prevents media from surviving an engine crash.
func protectProcess(cmd *exec.Cmd) (func() error, func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	return func() error { return nil }, func() {}, nil
}
