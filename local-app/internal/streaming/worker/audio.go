package worker

import (
	"context"
	"sync"

	"go.uber.org/zap"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/runtime/jobgroup"
)

type AudioWorker struct {
	jobs                    *jobgroup.Group
	mu                      sync.Mutex
	closing, backendStarted bool
	queue                   chan []byte
}

func NewAudioWorker() *AudioWorker {
	return &AudioWorker{
		jobs:  jobgroup.New(),
		queue: make(chan []byte, 128),
	}
}

func (w *AudioWorker) Start() error {
	return w.jobs.Start()
}

func (w *AudioWorker) Close() {
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	w.jobs.Close()
}

func (w *AudioWorker) StartMetricDeliveryLoop(send func(context.Context, []byte) error) error {
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
					logger.FromContext(ctx).Warn("metric delivery failed", zap.Error(e))
				}
			}
		}
	})
	if e == nil {
		w.backendStarted = true
	}
	return e
}

func (w *AudioWorker) EnqueueMetric(b []byte) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closing {
		return false
	}
	select {
	case w.queue <- b:
		return true
	default:
		return false
	}
}
