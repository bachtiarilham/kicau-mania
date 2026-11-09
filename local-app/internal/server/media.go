package server

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/infra/logger"
)

type mediaServer interface {
	Start(context.Context) error
	Done() <-chan struct{}
	Close()
}

// superviseMedia restarts media without shutting down the engine HTTP server.
func superviseMedia(ctx context.Context, server mediaServer, retry time.Duration) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ctx.Err() == nil {
			if err := server.Start(ctx); err != nil {
				logger.Warn("media unavailable; engine control remains available", zap.Error(err))
			} else {
				select {
				case <-ctx.Done():
				case <-server.Done():
					logger.Warn("media process stopped; restarting independently")
				}
				server.Close()
			}
			timer := time.NewTimer(retry)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	return done
}

func startIndependentModules(modules ...module) func() {
	started := []module{}
	for _, m := range modules {
		if err := m.Start(); err != nil {
			logger.Warn("media worker unavailable; engine control remains available", zap.Error(err))
			m.Close()
			continue
		}
		started = append(started, m)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, m := range started {
				m.Close()
			}
		})
	}
}
