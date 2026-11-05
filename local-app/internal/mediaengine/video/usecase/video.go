package usecase

import (
	channelModel "kicau.local/edge/internal/mediaengine/channel/model"
	"kicau.local/edge/internal/mediaengine/video/model"
)

type ChannelRepository interface {
	Channels() []channelModel.Channel
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

func (u *UseCase) Channels() []model.Channel {
	out := []model.Channel{}
	for _, ch := range u.sources.Channels() {
		out = append(out, model.Channel{
			ID:   ch.ID,
			WHEP: u.playback.WHEP(ch.ID),
		})
	}
	return out
}
