package worker

import (
	"context"
	"log/slog"
	"time"
)

// StartCaptureLoop runs capture in the background and reconnects after failures.
func (w *Worker) StartCaptureLoop(channel string, capture func(context.Context) error) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			_ = capture(ctx)
			if ctx.Err() != nil {
				return
			}
			slog.Warn("audio feed disconnected; retrying", "channel", channel)
			if !w.wait(ctx, 5*time.Second) {
				return
			}
		}
	})
}
