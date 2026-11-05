package worker

import (
	"context"
	"time"
)

func (w *Worker) RunGuard(check func(context.Context)) error {
	return w.jobs.Go(func(ctx context.Context) {
		for w.wait(ctx, 5*time.Second) {
			if ctx.Err() != nil {
				return
			}
			check(ctx)
		}
	})
}
