package worker

import (
	"context"
	"time"

	"kicau.local/edge/internal/apperror"
)

// StartUploadLoop schedules a background pass immediately, then waits five
// seconds between completed passes until the worker closes.
func (w *Worker) StartUploadLoop(upload func(context.Context)) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			upload(ctx)
			if !w.wait(ctx, 5*time.Second) {
				return
			}
		}
	})
}
// UploadWithTimeout runs one upload synchronously with a 30-minute deadline.
func (w *Worker) UploadWithTimeout(ctx context.Context, put func(context.Context) error) error {
	if w.jobs.Context().Err() != nil {
		return apperror.ErrUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	upload, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	return put(upload)
}
