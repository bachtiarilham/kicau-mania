package worker

import (
	"context"
	"sync"
	"time"

	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type SessionWorker struct {
	jobs *jobgroup.Group
}

func NewSessionWorker() *SessionWorker {
	return &SessionWorker{jobs: jobgroup.New()}
}

func (w *SessionWorker) Start() error {
	return w.jobs.Start()
}

func (w *SessionWorker) Context() context.Context {
	return w.jobs.Context()
}

func (w *SessionWorker) Run(f func(context.Context)) error {
	return w.jobs.Go(f)
}

func (w *SessionWorker) Group(ctx context.Context, tasks []func(context.Context), done func()) error {
	return w.jobs.Go(func(context.Context) {
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(f func(context.Context)) {
				defer wg.Done()
				f(ctx)
			}(task)
		}
		wg.Wait()
		done()
	})
}

func (w *SessionWorker) Wait(ctx context.Context, d time.Duration) bool {
	return jobgroup.Wait(ctx, d)
}

func (w *SessionWorker) Close() {
	w.jobs.Close()
}
