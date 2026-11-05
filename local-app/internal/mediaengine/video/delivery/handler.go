package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/mediaengine/video/model"
	"kicau.local/edge/internal/mediaengine/video/usecase"
)

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc}
}

func (h *Handler) channels(c *gin.Context) {
	records := h.uc.Channels()
	out := make([]model.ChannelResponse, 0, len(records))
	for _, ch := range records {
		out = append(out, model.ChannelResponse{
			ID:    ch.ID,
			WHEP:  ch.WHEP,
			Audio: "/api/v1/channels/" + ch.ID + "/audio",
		})
	}
	c.JSON(200, gin.H{"data": out})
}
