package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/streaming/model"
)

// RestreamUseCase contains only the restream operations exposed by this handler.
type RestreamUseCase interface {
	Restreams() []model.RestreamView
	StartRestream(string, string) error
	StopRestream(string) error
}

type Handler struct{ uc RestreamUseCase }

func New(uc RestreamUseCase) *Handler {
	return &Handler{uc}
}

func (h *Handler) list(c *gin.Context) {
	response.Success(c, "Restreams retrieved", h.uc.Restreams())
}

func (h *Handler) start(c *gin.Context) {
	var req model.StartRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	if e := h.uc.StartRestream(req.Channel, req.Target); e != nil {
		response.WriteError(c, e)
		return
	}
	response.Accepted(c, "Restream requested", nil)
}

func (h *Handler) stop(c *gin.Context) {
	if e := h.uc.StopRestream(c.Param("target")); e != nil {
		response.WriteError(c, e)
		return
	}
	response.NoContent(c)
}
