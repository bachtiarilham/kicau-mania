// Limits stop recordings when disk capacity or duration limits are reached.
package usecase

import (
	"context"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/mediaengine/media"
)

func (u *UseCase) checkLimits(ctx context.Context) {
	free, err := u.disk.Free(u.cfg.DataDir)
	u.mu.Lock()
	records, readErr := u.repo.List(ctx)
	expired := map[string]bool{}
	if readErr == nil {
		for _, r := range records {
			if r.State == "RECORDING" && time.Since(r.StartedAt) >= u.cfg.MaxRound {
				expired[r.RoundID] = true
			}
		}
	}
	processes := []media.Process{}
	for _, a := range u.running {
		if err != nil || free < u.cfg.MinFreeBytes || expired[a.round] {
			logger.Warn("stopping recording: capacity or time limit", zap.String("round", a.round))
			processes = append(processes, a.p)
		}
	}
	u.mu.Unlock()
	if err := u.executor.StopProcesses(processes); err != nil && ctx.Err() == nil {
		logger.Error("cannot schedule recording stop", zap.Error(err))
	}
}
