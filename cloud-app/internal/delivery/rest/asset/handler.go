package asset

import (
	"context"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	Lookup(context.Context, string) (model.Record, error)
	Transfer(context.Context, string, string, string, string) (model.Record, error)
}
type Handler struct{ uc UseCase }

func New(u UseCase) *Handler { return &Handler{u} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/birds/lookup", func(c *gin.Context) {
		row, e := h.uc.Lookup(c.Request.Context(), c.Query("ring_number"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Bird retrieved", row)
	})
	g.POST("/birds/:id/transfer", func(c *gin.Context) {
		var in struct {
			Owner  string `json:"to_owner_id"`
			Reason string `json:"reason"`
		}
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.Transfer(c.Request.Context(), request.Actor(c), c.Param("id"), in.Owner, in.Reason)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Owner transferred", row)
	})
}
