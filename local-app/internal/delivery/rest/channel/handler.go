package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/channel/model"
)

// ChannelUseCase provides the channel data needed by this handler.
type ChannelUseCase interface {
	Channels() []model.PlaybackChannel
}

// ChannelResponse is the REST representation of a playable channel.
type ChannelResponse struct {
	ID    string `json:"id"`
	WHEP  string `json:"whep_url"`
	Audio string `json:"audio_path"`
}

type Handler struct {
	uc ChannelUseCase
}

func New(uc ChannelUseCase) *Handler {
	return &Handler{uc}
}

func (h *Handler) channels(c *gin.Context) {
	records := h.uc.Channels()
	out := make([]ChannelResponse, 0, len(records))
	for _, ch := range records {
		out = append(out, ChannelResponse{
			ID:    ch.ID,
			WHEP:  ch.WHEP,
			Audio: "/api/v1/channels/" + ch.ID + "/audio",
		})
	}
	response.Success(c, "Channels retrieved", out)
}
