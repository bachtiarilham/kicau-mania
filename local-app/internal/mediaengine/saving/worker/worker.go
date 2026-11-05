package worker

import (
	"context"
	"time"

	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type Worker struct {
	jobs *jobgroup.Group
	wait func(context.Context, time.Duration) bool
}

func New() *Worker                         { return &Worker{jobs: jobgroup.New(), wait: jobgroup.Wait} }
func (w *Worker) Start() error             { return w.jobs.Start() }
func (w *Worker) Context() context.Context { return w.jobs.Context() }
func (w *Worker) Close()                   { w.jobs.Close() }
