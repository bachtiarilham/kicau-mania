// Package worker owns goroutines and cancellation waits for media session jobs.
package worker

import (
	"context"
	"kicau.local/edge/internal/infra/runtime/jobgroup"
	"sync"
	"time"
)

type Worker struct{ jobs *jobgroup.Group }

func New() *Worker                                  { return &Worker{jobs: jobgroup.New()} }
func (w *Worker) Start() error                      { return w.jobs.Start() }
func (w *Worker) Context() context.Context          { return w.jobs.Context() }
func (w *Worker) Run(f func(context.Context)) error { return w.jobs.Go(f) }
func (w *Worker) Group(ctx context.Context, tasks []func(context.Context), done func()) error {
	return w.jobs.Go(func(context.Context) {
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(f func(context.Context)) { defer wg.Done(); f(ctx) }(task)
		}
		wg.Wait()
		done()
	})
}
func (w *Worker) Wait(ctx context.Context, d time.Duration) bool { return jobgroup.Wait(ctx, d) }
func (w *Worker) Close()                                         { w.jobs.Close() }
