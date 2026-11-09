// Package worker schedules engine communication independently of media processing.
package worker

import (
	"context"
	"go.uber.org/zap"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/runtime/jobgroup"
	"time"
)

type Worker struct{ jobs *jobgroup.Group }

func New() *Worker             { return &Worker{jobs: jobgroup.New()} }
func (w *Worker) Start() error { return w.jobs.Start() }
func (w *Worker) Close()       { w.jobs.Close() }
func (w *Worker) Loop(name string, interval time.Duration, run func(context.Context) error) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			if err := run(ctx); err != nil && ctx.Err() == nil {
				logger.FromContext(ctx).Warn("engine communication pending", zap.String("task", name), zap.Error(err))
			}
			if !jobgroup.Wait(ctx, interval) {
				return
			}
		}
	})
}
