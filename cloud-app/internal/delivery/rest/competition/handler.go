package competition

import (
	"context"

	"github.com/gin-gonic/gin"

	input "kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	Registration(context.Context, string, string) (model.Record, error)
	Registrations(context.Context, string, string, string, int) ([]model.Record, error)
	CreateRegistration(context.Context, string, input.Registration) (model.Record, error)
	UpdateRegistration(context.Context, string, string, input.Registration) (model.Record, error)
	CancelRegistration(context.Context, string, string) (model.Record, error)
	CreatePayment(context.Context, string, string, input.Payment) (model.Record, error)
	VerifyPayment(context.Context, string, string, []input.Allocation) (model.Record, error)
	RejectPayment(context.Context, string, string) (model.Record, error)
	RoundAction(context.Context, string, string, string, string) (model.Record, error)
	Recap(context.Context, string, string) ([]model.Record, error)
	Judges(context.Context, string, string) ([]model.Record, error)
	AssignJudge(context.Context, string, string, string) ([]model.Record, error)
	RemoveJudge(context.Context, string, string, string) error
	Recordings(context.Context, string, string) ([]model.Record, error)
}
type Reports interface {
	Get(context.Context, string, string, string) (model.Record, error)
}
type Handler struct {
	uc      UseCase
	reports Reports
}

func New(u UseCase, r Reports) *Handler { return &Handler{u, r} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/registrations", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		rows, e := h.uc.Registrations(c.Request.Context(), request.Actor(c), c.Query("arena_id"), cursor, limit)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registrations retrieved", request.PageData(rows, limit))
	})
	g.POST("/registrations", func(c *gin.Context) {
		var in input.Registration
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.CreateRegistration(c.Request.Context(), request.Actor(c), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Registration created", row)
	})
	g.GET("/registrations/:id", func(c *gin.Context) {
		row, e := h.uc.Registration(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration retrieved", row)
	})
	g.PUT("/registrations/:id", func(c *gin.Context) {
		var in input.Registration
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.UpdateRegistration(c.Request.Context(), request.Actor(c), c.Param("id"), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration updated", row)
	})
	g.POST("/registrations/:id/cancel", func(c *gin.Context) {
		row, e := h.uc.CancelRegistration(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration cancelled", row)
	})
	g.POST("/registrations/:id/payments", func(c *gin.Context) {
		var in input.Payment
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.CreatePayment(c.Request.Context(), request.Actor(c), c.Param("id"), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Payment submitted", row)
	})
	g.POST("/payments/:id/verify", func(c *gin.Context) {
		var in struct {
			Allocations []input.Allocation `json:"allocations"`
		}
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.VerifyPayment(c.Request.Context(), request.Actor(c), c.Param("id"), in.Allocations)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Payment verified and participants confirmed", row)
	})
	g.POST("/payments/:id/reject", func(c *gin.Context) {
		row, e := h.uc.RejectPayment(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Payment rejected", row)
	})
	for _, action := range []string{"start", "begin-judging", "interrupt", "resume", "cancel", "close"} {
		action := action
		g.POST("/rounds/:id/"+action, func(c *gin.Context) {
			winner := ""
			if action == "close" {
				var in struct {
					Winner string `json:"winner_registration_item_id"`
				}
				if !request.Bind(c, &in) {
					return
				}
				winner = in.Winner
			}
			row, e := h.uc.RoundAction(c.Request.Context(), request.Actor(c), c.Param("id"), action, winner)
			if e != nil {
				response.WriteError(c, e)
				return
			}
			response.Success(c, "Round updated", row)
		})
	}
	g.GET("/rounds/:id/recap", func(c *gin.Context) {
		rows, e := h.uc.Recap(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Live recap retrieved", rows)
	})
	g.GET("/rounds/:id/judges", func(c *gin.Context) {
		rows, e := h.uc.Judges(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Judges retrieved", rows)
	})
	g.POST("/rounds/:id/judges", func(c *gin.Context) {
		var in struct {
			User string `json:"user_id"`
		}
		if !request.Bind(c, &in) {
			return
		}
		rows, e := h.uc.AssignJudge(c.Request.Context(), request.Actor(c), c.Param("id"), in.User)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Judge assigned", rows)
	})
	g.DELETE("/rounds/:id/judges/:assignment", func(c *gin.Context) {
		if e := h.uc.RemoveJudge(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("assignment")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	g.GET("/arenas/:arena/reports/dashboard", func(c *gin.Context) {
		row, e := h.reports.Get(c.Request.Context(), request.Actor(c), c.Param("arena"), "")
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Dashboard snapshot retrieved", row)
	})
	g.GET("/arenas/:arena/reports/rounds/:round", func(c *gin.Context) {
		row, e := h.reports.Get(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("round"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Score snapshot retrieved", row)
	})
	g.GET("/registrations/:id/recordings", func(c *gin.Context) {
		rows, e := h.uc.Recordings(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Recording metadata retrieved", rows)
	})
}
