// Upload processes persisted ready recordings and saves retry outcomes.
package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/infra/logger"
)

func (u *UseCase) uploadPending(ctx context.Context) {
	u.mu.Lock()
	records, err := u.repo.List(ctx)
	u.mu.Unlock()
	if err != nil {
		if ctx.Err() == nil {
			logger.Error("upload queue read failed", zap.Error(err))
		}
		return
	}
	activeRounds := map[string]bool{}
	for _, r := range records {
		if r.State == "RECORDING" {
			activeRounds[r.RoundID] = true
		}
	}
	for _, r := range records {
		if ctx.Err() != nil {
			return
		}
		if r.State != "READY" || activeRounds[r.RoundID] || time.Now().Before(r.NextAttempt) {
			continue
		}
		err = u.executor.UploadWithTimeout(ctx, func(uploadCtx context.Context) error { return u.storage.Put(uploadCtx, r.Key, r.File) })
		if ctx.Err() != nil {
			return
		}
		r.Attempts++
		if err == nil {
			r.State = "UPLOADED"
		} else {
			shift := min(r.Attempts, 8)
			r.NextAttempt = time.Now().Add(time.Duration(1<<shift) * time.Second)
			logger.Warn("recording upload failed; queued for retry", zap.String("recording", r.ID), zap.Error(err))
		}
		u.mu.Lock()
		err = u.repo.Save(ctx, r)
		u.mu.Unlock()
		if err != nil {
			logger.Error("upload state persistence failed", zap.String("recording", r.ID), zap.Error(err))
		}
	}
}
