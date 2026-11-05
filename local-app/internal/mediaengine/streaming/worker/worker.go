package worker

import (
	"context"
	"sync"
	"time"

	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type task struct {
	cancel context.CancelFunc
	done   chan struct{}
}
type Worker struct {
	jobs    *jobgroup.Group
	wait    func(context.Context, time.Duration) bool
	mu      sync.Mutex
	tasks   map[string]*task
	closing bool
}

func New() *Worker {
	return &Worker{jobs: jobgroup.New(), wait: jobgroup.Wait, tasks: map[string]*task{}}
}
func (w *Worker) Start() error { return w.jobs.Start() }
func (w *Worker) Close() {
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	w.jobs.Close()
	w.mu.Lock()
	clear(w.tasks)
	w.mu.Unlock()
}
