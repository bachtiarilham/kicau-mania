// Package usecase builds the public channel catalog from identities and playback addresses.
package usecase

import (
	"kicau.local/edge/internal/mediaengine/channel/model"
)

type ChannelRepository interface {
	Channels() []model.Channel
}

type Playback interface {
	WHEP(string) string
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
		out = append(out, model.PlaybackChannel{
			ID:   ch.ID,
			WHEP: u.playback.WHEP(ch.ID),
		})
	}
	return out
}
