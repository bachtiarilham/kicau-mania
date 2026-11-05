package worker

import (
	"context"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/infra/logger"
)

// StartCaptureLoop runs capture in the background and reconnects after failures.
func (w *Worker) StartCaptureLoop(channel string, capture func(context.Context) error) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			_ = capture(ctx)
			if ctx.Err() != nil {
				return
			}
			logger.Warn("audio feed disconnected; retrying", zap.String("channel", channel))
			if !w.wait(ctx, 5*time.Second) {
				return
			}
		}
	})
}
