package subprocess

import (
	"os"
	"syscall"
)

func stopMediaServer(p *os.Process) error { return p.Signal(syscall.SIGINT) }
