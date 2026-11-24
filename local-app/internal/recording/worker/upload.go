package worker

import (
	"context"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type UploadWorker struct {
	jobs *jobgroup.Group
	wait func(context.Context, time.Duration) bool
}

func NewUploadWorker() *UploadWorker {
	return &UploadWorker{
		jobs: jobgroup.New(),
		wait: jobgroup.Wait,
	}
}

func (w *UploadWorker) Start() error {
	return w.jobs.Start()
}

func (w *UploadWorker) Context() context.Context {
	return w.jobs.Context()
}

func (w *UploadWorker) Close() {
	w.jobs.Close()
}

func (w *UploadWorker) StartUploadLoop(upload func(context.Context)) error {
	return w.jobs.Go(func(ctx context.Context) {
		for ctx.Err() == nil {
			upload(ctx)
			if !w.wait(ctx, 5*time.Second) {
				return
			}
		}
	})
}

func (w *UploadWorker) UploadWithTimeout(ctx context.Context, put func(context.Context) error) error {
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
