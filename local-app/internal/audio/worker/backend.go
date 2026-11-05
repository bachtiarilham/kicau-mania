package worker

import (
	"context"
	"log/slog"

	"kicau.local/edge/internal/apperror"
)

// StartMetricDeliveryLoop starts one background consumer for queued metrics.
func (w *Worker) StartMetricDeliveryLoop(send func(context.Context, []byte) error) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return apperror.ErrUnavailable
	}
	if w.backendStarted {
		return apperror.ErrConflict
	}
	e := w.jobs.Go(func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case b := <-w.queue:
				if ctx.Err() != nil {
					return
				}
				if e := send(ctx, b); e != nil && ctx.Err() == nil {
					slog.Warn("metric delivery failed")
				}
			}
		}
	})
	if e == nil {
		w.backendStarted = true
	}
	return e
}

// EnqueueMetric is best effort; a slow backend never blocks PCM capture.
func (w *Worker) EnqueueMetric(b []byte) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return false
	}
	select {
	case w.queue <- append([]byte(nil), b...):
		return true
	default:
		return false
	}
}
