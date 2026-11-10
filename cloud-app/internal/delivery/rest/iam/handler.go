// Package iam adapts account and role-management HTTP requests to authorization usecases.
package iam

import (
	"context"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	RemoveGlobal(context.Context, string, string, string) error
	Users(context.Context, string, string, int) ([]model.Record, error)
	Lookup(context.Context, string, string, string) (model.Record, error)
	Roles(context.Context) ([]model.Record, error)
	Assignments(context.Context, string, string) ([]model.Record, error)
	Grant(context.Context, string, string, string, string) error
	Revoke(context.Context, string, string, string) error
	PatchProfile(context.Context, string, model.Record) (model.Record, error)
	SetActive(context.Context, string, string, bool) (model.Record, error)
}
type Handler struct{ uc UseCase }

func New(u UseCase) *Handler { return &Handler{u} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.DELETE("/users/:id/roles/:role", func(c *gin.Context) {
		if e := h.uc.RemoveGlobal(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("role")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	g.GET("/users", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		rows, e := h.uc.Users(c.Request.Context(), request.Actor(c), cursor, limit)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Users retrieved", request.PageData(rows, limit))
	})
	g.GET("/users/lookup", func(c *gin.Context) {
		row, e := h.uc.Lookup(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Query("email"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "User retrieved", row)
	})
	g.GET("/roles", func(c *gin.Context) {
		rows, e := h.uc.Roles(c.Request.Context())
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Roles retrieved", rows)
	})
	g.PATCH("/auth/me", func(c *gin.Context) {
		var fields model.Record
		if !request.Bind(c, &fields) {
			return
		}
		row, e := h.uc.PatchProfile(c.Request.Context(), request.Actor(c), fields)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Profile updated", row)
	})
	g.PATCH("/users/:id/active", func(c *gin.Context) {
		var in struct {
			Active *bool `json:"is_active"`
		}
		if !request.Bind(c, &in) {
			return
		}
		if in.Active == nil {
			response.BadRequest(c, "is_active is required", nil)
			return
		}
		row, e := h.uc.SetActive(c.Request.Context(), request.Actor(c), c.Param("id"), *in.Active)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "User updated", row)
	})
	g.POST("/users/:id/roles", func(c *gin.Context) {
		var in struct {
			Role string `json:"role"`
		}
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.Grant(c.Request.Context(), request.Actor(c), "", c.Param("id"), in.Role); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	g.GET("/arenas/:arena/assignments", func(c *gin.Context) {
		rows, e := h.uc.Assignments(c.Request.Context(), request.Actor(c), c.Param("arena"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Assignments retrieved", rows)
	})
	g.POST("/arenas/:arena/assignments", func(c *gin.Context) {
		var in struct {
			UserID string `json:"user_id"`
			Role   string `json:"role"`
		}
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.Grant(c.Request.Context(), request.Actor(c), c.Param("arena"), in.UserID, in.Role); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	g.DELETE("/arenas/:arena/assignments/:assignment", func(c *gin.Context) {
		if e := h.uc.Revoke(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("assignment")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
}
