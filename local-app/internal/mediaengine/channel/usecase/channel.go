// Package usecase builds the public channel catalog from identities and playback addresses.
package usecase

import (
	"kicau.local/edge/internal/mediaengine/channel/model"
)

type ChannelRepository interface {
	Channels() []model.Channel
}

type Playback interface {
	HubRTSP(string) (string, error)
}

type UseCase struct {
	sources  ChannelRepository
	playback Playback
}

func New(
	sources ChannelRepository,
	playback Playback,
) *UseCase {
	return &UseCase{
		sources:  sources,
		playback: playback,
	}
}

func (u *UseCase) Channels() []model.PlaybackChannel {
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
