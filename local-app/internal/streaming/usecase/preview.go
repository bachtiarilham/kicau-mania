package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/streaming/model"
)

func (u *AudioUseCase) SubscribePreviewMetrics() (<-chan []byte, func(), error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.started || u.closing {
		return nil, nil, apperror.ErrUnavailable
	}
	messages, unsubscribe := u.broadcaster.Subscribe("preview-metrics:")
	u.previewSubscribers++
	u.startPreview()
	return messages, sync.OnceFunc(func() {
		u.mu.Lock()
		defer u.mu.Unlock()
		unsubscribe()
		u.previewSubscribers--
		if u.previewSubscribers == 0 {
			u.stopPreview()
		}
	}), nil
}

func (u *AudioUseCase) startPreview() {
	if u.previewCancel != nil || len(u.active) > 0 || len(u.opts.PreviewSources) == 0 || u.opts.PreviewCapture == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.previewCancel = cancel
	for id, source := range u.opts.PreviewSources {
		id := id
		source := source
		u.previewWorkers.Go(func() {
			for ctx.Err() == nil {
				_ = u.opts.PreviewCapture(ctx, source, func(pcm []byte) {
					u.mu.Lock()
					defer u.mu.Unlock()
					if ctx.Err() == nil {
						metric := model.Analyze(id, pcm, 16000, u.opts.Threshold, time.Now())
						if data, err := json.Marshal(metric); err == nil {
							u.broadcaster.Publish("preview-metrics:", data)
						}
					}
				})
				timer := time.NewTimer(5 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
			}
		})
	}
}

func (u *AudioUseCase) stopPreview() {
	if u.previewCancel != nil {
		u.previewCancel()
		u.previewCancel = nil
	}
}
