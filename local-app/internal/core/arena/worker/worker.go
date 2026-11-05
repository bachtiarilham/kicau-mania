// Package worker schedules non-critical sync/backup callbacks independently of HTTP.
package worker

import (
	"context"
	"log/slog"
	"time"

	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type Worker struct{ jobs *jobgroup.Group }

func New() *Worker             { return &Worker{jobs: jobgroup.New()} }
func (w *Worker) Start() error { return w.jobs.Start() }
func (w *Worker) Close()       { w.jobs.Close() }
func (w *Worker) Loop(name string, interval time.Duration, run func(context.Context) error) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			if err := run(ctx); err != nil && ctx.Err() == nil {
				slog.Warn("arena background task pending", "task", name)
			}
			if !jobgroup.Wait(ctx, interval) {
				return
			}
		}
	})
}
