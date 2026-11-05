package worker

import (
	"context"

	"kicau.local/edge/internal/media"
	"kicau.local/edge/internal/runtime/jobgroup"
)

func (w *Worker) Watch(p media.Process, complete func(error)) (<-chan struct{}, error) {
	done := make(chan struct{})
	e := w.jobs.Go(func(ctx context.Context) { defer close(done); err := jobgroup.WaitProcess(ctx, p); complete(err) })
	if e != nil {
		return nil, e
	}
	return done, nil
}
func (w *Worker) StopProcesses(items []media.Process) error {
	for _, p := range items {
		p := p
		if e := w.jobs.Go(func(context.Context) { p.Stop() }); e != nil {
			return e
		}
	}
	return nil
}
func (w *Worker) WaitFor(ctx context.Context, completions []<-chan struct{}) error {
	for _, done := range completions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
		}
	}
	return nil
}
