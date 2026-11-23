package usecase

import (
	"context"

	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
)

type Options struct {
	SampleRate     int
	Threshold      float64
	PreviewSources map[string]string
	PreviewCapture func(context.Context, string, func([]byte)) error
}

type ChannelRepository interface {
	HubRTSP(string) (string, error)
}

type Broadcaster interface {
	Publish(string, []byte)
	Subscribe(string) (<-chan []byte, func())
	Reset()
}

type MetricSender interface {
	Send(context.Context, []byte) error
}

type Executor interface {
	Start() error

	StartMetricDeliveryLoop(func(context.Context, []byte) error) error
	EnqueueMetric([]byte) bool
	Close()
}

type UseCase struct {
	opts    Options
	sources ChannelRepository

	broadcaster        Broadcaster
	backend            MetricSender
	executor           Executor
	mu                 sync.Mutex
	active             map[string]bool
	activeAt           time.Time
	activeContext      context.Context
	activeCancel       context.CancelFunc
	started, closing   bool
	closeOnce          sync.Once
	previewSubscribers int
	previewCancel      context.CancelFunc
	previewWorkers     sync.WaitGroup
}

func New(
	opts Options,
	sources ChannelRepository,

	broadcaster Broadcaster,
	backend MetricSender,
	executor Executor,
) *UseCase {
	return &UseCase{
		opts:    opts,
		sources: sources,

		broadcaster: broadcaster,
		backend:     backend,
		executor:    executor,
	}
}
func (u *UseCase) Start() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closing {
		return apperror.ErrUnavailable
	}
	if u.started {
		return apperror.ErrConflict
	}
	if err := u.executor.Start(); err != nil {
		return err
	}
	if u.backend != nil {
		if err := u.executor.StartMetricDeliveryLoop(u.sendMetric); err != nil {
			u.closing = true
			u.executor.Close()
			return err
		}
	}
	u.started = true
	return nil
}

// SubscribeAudio validates the channel before subscribing to its PCM frames.
func (u *UseCase) SubscribeAudio(channelID string) (<-chan []byte, func(), error) {
	if _, err := u.sources.HubRTSP(channelID); err != nil {
		return nil, nil, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.active[channelID] {
		return nil, nil, apperror.ErrConflict
	}
	messages, cancel := u.broadcaster.Subscribe("audio:" + channelID)
	return messages, cancel, nil
}

// SubscribeMetrics subscribes to metrics from all configured channels.
func (u *UseCase) SubscribeMetrics() (<-chan []byte, func(), error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	messages, cancel := u.broadcaster.Subscribe("metrics:")
	return messages, cancel, nil
}

func (u *UseCase) Close() {
	u.Deactivate()
	u.closeOnce.Do(func() {
		u.mu.Lock()
		u.closing = true
		u.stopPreview()
		u.mu.Unlock()
		u.previewWorkers.Wait()
		u.executor.Close()
	})
}
