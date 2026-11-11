package worker

import (
	"sync"

	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type Worker struct {
	jobs                    *jobgroup.Group
	mu                      sync.Mutex
	closing, backendStarted bool
	queue                   chan []byte
}

func New() *Worker {
	return &Worker{jobs: jobgroup.New(), queue: make(chan []byte, 128)}
}
func (w *Worker) Start() error { return w.jobs.Start() }
func (w *Worker) Close()       { w.mu.Lock(); w.closing = true; w.mu.Unlock(); w.jobs.Close() }
