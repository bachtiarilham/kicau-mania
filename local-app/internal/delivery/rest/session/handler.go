// Package delivery exposes the unified media session controls.
package delivery

import (
	"context"
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/session/model"
)

type SessionUseCase interface {
	StartSession(context.Context, model.StartRequest) (model.Session, error)
	StopSession(context.Context, string) (model.Session, error)
	Get(context.Context, string) (model.Session, error)
}
type Handler struct{ uc SessionUseCase }

func New(uc SessionUseCase) *Handler { return &Handler{uc: uc} }
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/media/sessions", h.start)
	g.POST("/media/sessions/:id/stop", h.stop)
	g.GET("/media/sessions/:id", h.get)
}
func (h *Handler) start(c *gin.Context) {
	var req model.StartRequest
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	s, err := h.uc.StartSession(c.Request.Context(), req)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Accepted(c, "Media session accepted", s)
}
func (h *Handler) stop(c *gin.Context) {
	s, err := h.uc.StopSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Accepted(c, "Media session stop accepted", s)
}
func (h *Handler) get(c *gin.Context) {
	s, err := h.uc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Media session retrieved", s)
}
