package subprocess

import "os"

// MediaMTX owns hook processes in Windows Job Objects with KILL_ON_JOB_CLOSE.
// Terminating the hub closes those handles and terminates its hook children.
func stopMediaServer(p *os.Process) error { return p.Kill() }
