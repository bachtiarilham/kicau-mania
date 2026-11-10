// Package worker schedules report maintenance independently of HTTP requests.
package worker

import (
	"context"
	"sync"
	"time"
)

type Worker struct {
	wake   chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func New() *Worker { return &Worker{wake: make(chan struct{}, 1), done: make(chan struct{})} }
func (w *Worker) Notify() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}
func (w *Worker) Start(parent context.Context, interval, timeout time.Duration, refresh func(context.Context)) {
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		run := func() { job, stop := context.WithTimeout(ctx, timeout); defer stop(); refresh(job) }
		run()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			case <-w.wake:
				timer := time.NewTimer(500 * time.Millisecond)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				select {
				case <-w.wake:
				default:
				}
				run()
			}
		}
	}()
}
func (w *Worker) Close() {
	w.once.Do(func() {
		if w.cancel != nil {
			w.cancel()
			<-w.done
		}
	})
}
