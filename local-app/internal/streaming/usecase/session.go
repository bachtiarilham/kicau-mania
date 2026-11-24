package usecase

import (
	"context"
	"encoding/json"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/streaming/model"
)

func (u *AudioUseCase) Activate(channels []string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.started || u.closing || len(u.active) > 0 {
		return apperror.ErrConflict
	}
	u.stopPreview()
	u.active = map[string]bool{}
	for _, id := range channels {
		u.active[id] = true
	}
	u.activeAt = time.Now()
	u.activeContext, u.activeCancel = context.WithCancel(context.Background())
	return nil
}

func (u *AudioUseCase) Deactivate() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.activeCancel != nil {
		u.activeCancel()
	}
	u.active = nil
	u.broadcaster.Reset()
}

func (u *AudioUseCase) Ingest(id string, pcm []byte) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.active[id] || u.closing {
		return
	}
	rate := u.opts.SampleRate
	if rate <= 0 {
		rate = 16000
	}
	metric := model.Analyze(id, pcm, rate, u.opts.Threshold, time.Now())
	b, err := json.Marshal(metric)
	if err != nil {
		return
	}
	u.broadcaster.Publish("audio:"+id, pcm)
	u.broadcaster.Publish("metrics:", b)
	u.broadcaster.Publish("preview-metrics:", b)
	if u.backend != nil {
		u.executor.EnqueueMetric(b)
	}
}

func (u *AudioUseCase) sendMetric(ctx context.Context, b []byte) error {
	var metric model.Metric
	if json.Unmarshal(b, &metric) != nil {
		return apperror.ErrInvalid
	}
	u.mu.Lock()
	if !u.active[metric.ChannelID] || metric.At.Before(u.activeAt) {
		u.mu.Unlock()
		return nil
	}
	sessionContext := u.activeContext
	u.mu.Unlock()
	sendCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(sessionContext, cancel)
	defer stop()
	return u.backend.Send(sendCtx, b)
}
