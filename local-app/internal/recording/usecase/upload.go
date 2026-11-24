package usecase

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/recording/model"
)

func (u *SavingUseCase) uploadPending(ctx context.Context) {
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
		if r.State == "RECORDING" || r.State == "FINALIZING" {
			activeRounds[r.RoundID] = true
		}
	}
	if len(activeRounds) > 0 {
		return
	}
	for _, r := range records {
		if ctx.Err() != nil {
			return
		}
		if r.State != "READY" || time.Now().Before(r.NextAttempt) {
			continue
		}
		err = u.executor.UploadWithTimeout(ctx, func(uploadCtx context.Context) error {
			if cloud, ok := u.storage.(interface {
				PutRecording(context.Context, model.Recording) error
			}); ok {
				return cloud.PutRecording(uploadCtx, r)
			}
			return u.storage.Put(uploadCtx, r.Key, r.File)
		})
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, model.ErrNotMapped) {
			continue
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
