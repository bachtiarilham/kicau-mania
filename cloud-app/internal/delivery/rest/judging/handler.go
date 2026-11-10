package judging

import (
	"context"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
	input "kicau.cloud/control/internal/judging/model"
)

type UseCase interface {
	Rounds(context.Context, string, string, int) ([]model.Record, error)
	Participants(context.Context, string, string) ([]model.Record, error)
	Assessment(context.Context, string, string, string) (model.Record, error)
	History(context.Context, string, string, string) ([]model.Record, error)
	Submit(context.Context, string, string, string, input.Submit, bool) (model.Record, error)
}
type Handler struct{ uc UseCase }

func New(u UseCase) *Handler { return &Handler{u} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/judging/rounds", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		rows, e := h.uc.Rounds(c.Request.Context(), request.Actor(c), cursor, limit)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Assignments retrieved", request.PageData(rows, limit))
	})
	g.GET("/judging/rounds/:round/participants", func(c *gin.Context) {
		rows, e := h.uc.Participants(c.Request.Context(), request.Actor(c), c.Param("round"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Participants retrieved", rows)
	})
	path := "/judging/rounds/:round/participants/:item/assessment"
	g.GET(path, func(c *gin.Context) {
		row, e := h.uc.Assessment(c.Request.Context(), request.Actor(c), c.Param("round"), c.Param("item"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Assessment retrieved", row)
	})
	g.GET(path+"/history", func(c *gin.Context) {
		rows, e := h.uc.History(c.Request.Context(), request.Actor(c), c.Param("round"), c.Param("item"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "History retrieved", rows)
	})
	save := func(revision bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			var in input.Submit
			if !request.Bind(c, &in) {
				return
			}
			row, e := h.uc.Submit(c.Request.Context(), request.Actor(c), c.Param("round"), c.Param("item"), in, revision)
			if e != nil {
				response.WriteError(c, e)
				return
			}
			if revision {
				response.Success(c, "Assessment revised", row)
			} else {
				response.Created(c, "Assessment submitted", row)
			}
		}
	}
	g.POST(path, save(false))
	g.PUT(path, save(true))
}
