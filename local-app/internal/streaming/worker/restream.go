package worker

import (
	"context"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/media"
	"kicau.local/edge/internal/runtime/jobgroup"
)

// notify reports process observations. The usecase translates them to domain states.
func (w *Worker) Launch(target string, start func(context.Context) (media.Process, error), notify func(bool)) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return apperror.ErrUnavailable
	}
	if _, ok := w.tasks[target]; ok {
		return apperror.ErrConflict
	}
	ctx, cancel := context.WithCancel(w.jobs.Context())
	t := &task{cancel: cancel, done: make(chan struct{})}
	e := w.jobs.Go(func(context.Context) {
		defer close(t.done)
		for ctx.Err() == nil {
			p, e := start(ctx)
			notify(e == nil)
			if e == nil {
				_ = jobgroup.WaitProcess(ctx, p)
			}
			notify(false)
			if ctx.Err() != nil {
				return
			}
			if !w.wait(ctx, 5*time.Second) {
				return
			}
		}
	})
	if e != nil {
		cancel()
		return e
	}
	w.tasks[target] = t
	return nil
}
func (w *Worker) Stop(target string) error {
	w.mu.Lock()
	t, ok := w.tasks[target]
	if !ok {
		w.mu.Unlock()
		return apperror.ErrNotFound
	}
	t.cancel()
	w.mu.Unlock()
	<-t.done
	w.mu.Lock()
	if w.tasks[target] == t {
		delete(w.tasks, target)
	}
	w.mu.Unlock()
	return nil
}
