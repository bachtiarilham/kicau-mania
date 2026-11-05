package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/http/response"
	"kicau.local/edge/internal/streaming/model"
	"kicau.local/edge/internal/streaming/usecase"
)

type Handler struct{ uc *usecase.UseCase }

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc}
}

func (h *Handler) list(c *gin.Context) {
	c.JSON(200, gin.H{"data": h.uc.Restreams()})
}

func (h *Handler) start(c *gin.Context) {
	var req model.StartRequest
	if e := c.ShouldBindJSON(&req); e != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	if e := h.uc.StartRestream(req.Channel, req.Target); e != nil {
		response.WriteError(c, e)
		return
	}
	c.Status(202)
}

func (h *Handler) stop(c *gin.Context) {
	if e := h.uc.StopRestream(c.Param("target")); e != nil {
		response.WriteError(c, e)
		return
	}
	c.Status(204)
}
