// Package jobgroup manages background job admission, cancellation, and shutdown.
//
// Job groups provide cancellation and admission control for local jobs.
// Domain workers supply scheduling; usecases supply business operations.
package jobgroup

import (
	"context"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/mediaengine/media"
)

type Group struct {
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.Mutex
	wg              sync.WaitGroup
	started, closed bool
	once            sync.Once
}

func New() *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{ctx: ctx, cancel: cancel}
}

func (g *Group) Context() context.Context {
	return g.ctx
}

func (g *Group) Start() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return apperror.ErrUnavailable
	}
	if g.started {
		return apperror.ErrConflict
	}
	g.started = true
	return nil
}

// Go admits a job before Close can begin waiting. f is always invoked asynchronously.
func (g *Group) Go(f func(context.Context)) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || !g.started {
		return apperror.ErrUnavailable
	}
	g.wg.Add(1)
	go func() { defer g.wg.Done(); f(g.ctx) }()
	return nil
}

func (g *Group) Close() {
	g.once.Do(func() { g.mu.Lock(); g.closed = true; g.cancel(); g.mu.Unlock(); g.wg.Wait() })
}

func Wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func WaitProcess(ctx context.Context, p media.Process) error {
	select {
	case <-p.Done():
	case <-ctx.Done():
		p.Stop()
		<-p.Done()
	}
	return p.Err()
}
