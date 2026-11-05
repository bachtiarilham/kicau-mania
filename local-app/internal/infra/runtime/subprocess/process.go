package subprocess

import (
	"context"
	"io"
	"os/exec"
	"sync"
	"time"

	"kicau.local/edge/internal/mediaengine/media"
)

type process struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
	done  chan struct{}
	err   error
	once  sync.Once
}

func (p *process) Done() <-chan struct{} { return p.done }
func (p *process) Err() error            { <-p.done; return p.err }
func (p *process) Stop() {
	p.once.Do(func() {
		select {
		case <-p.done:
			return
		default:
		}
		// FFmpeg flushes its MP4 trailer on q, on Windows as well as Linux.
		_, _ = io.WriteString(p.stdin, "q\n")
		select {
		case <-p.done:
		case <-time.After(15 * time.Second):
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
}

func Start(ctx context.Context, binary string, args []string, stdout io.Writer) (media.Process, error) {
	return start(ctx, binary, args, stdout, false)
}

// StartMediaServer requests a graceful MediaMTX shutdown on supported platforms.
func StartMediaServer(ctx context.Context, binary string, args []string, stdout io.Writer) (media.Process, error) {
	return start(ctx, binary, args, stdout, true)
}

func start(ctx context.Context, binary string, args []string, stdout io.Writer, mediaServer bool) (media.Process, error) {
	cmd := exec.CommandContext(ctx, binary, args...)
	if mediaServer {
		cmd.Cancel = func() error { return stopMediaServer(cmd.Process) }
		cmd.WaitDelay = 15 * time.Second
	}
	// Never echo command arguments or stderr: sources and destinations contain credentials.
	cmd.Stdout = stdout
	cmd.Stderr = io.Discard
	if mediaServer && stdout != nil {
		cmd.Stderr = stdout
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		return nil, e
	}
	if e = cmd.Start(); e != nil {
		in.Close()
		return nil, e
	}
	p := &process{cmd: cmd, stdin: in, done: make(chan struct{})}
	go func() { p.err = cmd.Wait(); in.Close(); close(p.done) }()
	return p, nil
}
