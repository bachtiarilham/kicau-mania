package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/mediaengine/audio/model"
	channelModel "kicau.local/edge/internal/mediaengine/channel/model"
)

type Options struct {
	SampleRate int
	Threshold  float64
}

type ChannelRepository interface {
	Channels() []channelModel.Channel
	HubRTSP(string) (string, error)
}

type AudioDecoder interface {
	Audio(context.Context, string, func([]byte)) error
}

type Broadcaster interface {
	Publish(string, []byte)
	Subscribe(string) (<-chan []byte, func())
}

type MetricSender interface {
	Send(context.Context, []byte) error
}

type Executor interface {
	Start() error
	StartCaptureLoop(string, func(context.Context) error) error
	StartMetricDeliveryLoop(func(context.Context, []byte) error) error
	EnqueueMetric([]byte) bool
	Close()
}

type UseCase struct {
	opts             Options
	sources          ChannelRepository
	decoder          AudioDecoder
	broadcaster      Broadcaster
	backend          MetricSender
	executor         Executor
	mu               sync.Mutex
	started, closing bool
	closeOnce        sync.Once
}

func New(
	opts Options,
	sources ChannelRepository,
	decoder AudioDecoder,
	broadcaster Broadcaster,
	backend MetricSender,
	executor Executor,
) *UseCase {
	return &UseCase{
		opts:        opts,
		sources:     sources,
		decoder:     decoder,
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
	for _, ch := range u.sources.Channels() {
		id := ch.ID
		if err := u.executor.StartCaptureLoop(id, func(ctx context.Context) error { return u.capture(ctx, id) }); err != nil {
			u.closing = true
			u.executor.Close()
			return err
		}
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
	messages, cancel := u.broadcaster.Subscribe("audio:" + channelID)
	return messages, cancel, nil
}

// SubscribeMetrics subscribes to metrics from all configured channels.
func (u *UseCase) SubscribeMetrics() (<-chan []byte, func()) {
	return u.broadcaster.Subscribe("metrics:")
}

func (u *UseCase) capture(ctx context.Context, id string) error {
	source, e := u.sources.HubRTSP(id)
	if e != nil {
		return e
	}
	return u.decoder.Audio(ctx, source, func(pcm []byte) {
		metric := model.Analyze(id, pcm, u.opts.SampleRate, u.opts.Threshold, time.Now())
		b, _ := json.Marshal(metric)
		u.broadcaster.Publish("audio:"+id, pcm)
		u.broadcaster.Publish("metrics:", b)
		if u.backend != nil {
			u.executor.EnqueueMetric(b)
		}
	})
}
func (u *UseCase) sendMetric(ctx context.Context, b []byte) error {
	return u.backend.Send(ctx, b)
}
func (u *UseCase) Close() {
	u.closeOnce.Do(func() { u.mu.Lock(); u.closing = true; u.mu.Unlock(); u.executor.Close() })
}
