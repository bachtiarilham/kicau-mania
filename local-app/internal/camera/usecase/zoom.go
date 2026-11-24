package usecase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"kicau.local/edge/internal/camera/model"
)

var (
	ErrNotFound = errors.New("zoom channel not found")
)

type ZoomRepository interface {
	Load() (model.ZoomMapping, error)
	Save(model.ZoomMapping) error
}

type CommandBuilder interface {
	BuildArgs(ch model.ZoomChannel, sourceRTSP, targetRTSP string) []string
}

type ZoomUseCase struct {
	mu      sync.RWMutex
	repo    ZoomRepository
	builder CommandBuilder
	mapping model.ZoomMapping
}

func NewZoomUseCase(repo ZoomRepository, builder CommandBuilder) (*ZoomUseCase, error) {
	mapping, err := repo.Load()
	if err != nil {
		return nil, fmt.Errorf("load zoom mapping: %w", err)
	}
	return &ZoomUseCase{
		repo:    repo,
		builder: builder,
		mapping: mapping,
	}, nil
}

func (u *ZoomUseCase) ListActiveChannels(ctx context.Context) []model.ZoomChannel {
	u.mu.RLock()
	defer u.mu.RUnlock()

	var active []model.ZoomChannel
	for _, ch := range u.mapping.ActiveChannels {
		if ch.Active {
			active = append(active, ch)
		}
	}
	return active
}

func (u *ZoomUseCase) GetChannel(ctx context.Context, id string) (model.ZoomChannel, error) {
	u.mu.RLock()
	defer u.mu.RUnlock()

	for _, ch := range u.mapping.ActiveChannels {
		if ch.ChannelID == id {
			return ch, nil
		}
	}
	return model.ZoomChannel{}, ErrNotFound
}

func (u *ZoomUseCase) UpdateCalibration(ctx context.Context, id string, crop model.CropConfig, perspective model.PerspectiveConfig) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	foundIndex := -1
	for i, ch := range u.mapping.ActiveChannels {
		if ch.ChannelID == id {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return ErrNotFound
	}

	updated := u.mapping.ActiveChannels[foundIndex]
	updated.Crop = crop
	updated.Perspective = perspective

	if err := updated.Validate(); err != nil {
		return err
	}

	u.mapping.ActiveChannels[foundIndex] = updated
	return u.repo.Save(u.mapping)
}

func (u *ZoomUseCase) BuildArgs(ctx context.Context, id string, baseRTSP string) ([]string, error) {
	ch, err := u.GetChannel(ctx, id)
	if err != nil {
		return nil, err
	}

	sourceRTSP := fmt.Sprintf("%s/%s", baseRTSP, ch.SourceCamera)
	targetRTSP := fmt.Sprintf("%s/%s_zoom", baseRTSP, ch.ChannelID)

	return u.builder.BuildArgs(ch, sourceRTSP, targetRTSP), nil
}
