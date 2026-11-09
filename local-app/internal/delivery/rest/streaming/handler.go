package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/streaming/model"
)

// RestreamUseCase contains only the restream operations exposed by this handler.
type RestreamUseCase interface {
	Restreams() []model.RestreamView
}

type Handler struct{ uc RestreamUseCase }

func New(uc RestreamUseCase) *Handler {
	return &Handler{uc}
}

func (h *Handler) list(c *gin.Context) {
	response.Success(c, "Restreams retrieved", h.uc.Restreams())
}
