package usecase

import (
	"kicau.local/edge/internal/camera/model"
)

type ChannelRepository interface {
	Channels() []model.Channel
}

type Playback interface {
	HubRTSP(string) (string, error)
}

type CameraUseCase struct {
	sources  ChannelRepository
	playback Playback
}

func NewCameraUseCase(sources ChannelRepository, playback Playback) *CameraUseCase {
	return &CameraUseCase{
		sources:  sources,
		playback: playback,
	}
}

func (u *CameraUseCase) Channels() []model.PlaybackChannel {
	out := []model.PlaybackChannel{}
	for _, ch := range u.sources.Channels() {
		address, err := u.playback.HubRTSP(ch.ID)
		if err != nil {
			continue
		}
		out = append(out, model.PlaybackChannel{
			ID:   ch.ID,
			RTSP: address,
		})
	}
	return out
}
