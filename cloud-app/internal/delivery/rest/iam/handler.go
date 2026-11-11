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
	// @Summary Lepas role global
	// @Tags IAM
	// @Description Superadmin; tidak boleh melepas role superadmin diri sendiri.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param role path string true "Role global."
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/users/{id}/roles/{role} [delete]
	g.DELETE("/users/:id/roles/:role", func(c *gin.Context) {
		if e := h.uc.RemoveGlobal(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("role")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	// @Summary Daftar pengguna aman
	// @Tags IAM
	// @Description Superadmin.
	// @Security BearerAuth
	// @Produce json
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/users [get]
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
	// @Summary Cari pengguna untuk penugasan
	// @Tags IAM
	// @Description Admin arena/superadmin; identitas minimum.
	// @Security BearerAuth
	// @Produce json
	// @Param email query string true "Email akun."
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/users/lookup [get]
	g.GET("/users/lookup", func(c *gin.Context) {
		row, e := h.uc.Lookup(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Query("email"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "User retrieved", row)
	})
	// @Summary Daftar role sistem
	// @Tags IAM
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/roles [get]
	g.GET("/roles", func(c *gin.Context) {
		rows, e := h.uc.Roles(c.Request.Context())
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Roles retrieved", rows)
	})
	// @Summary Ubah nama/telepon sendiri
	// @Tags Auth
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Accept json
	// @Param request body ProfilePatch true "JSON body"
	// @Produce json
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 429 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/auth/me [patch]
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
	// @Summary Aktif/nonaktifkan akun
	// @Tags IAM
	// @Description Superadmin; tidak boleh menonaktifkan diri sendiri.
	// @Security BearerAuth
	// @Accept json
	// @Param request body ActivePatch true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/users/{id}/active [patch]
	g.PATCH("/users/:id/active", func(c *gin.Context) {
		var in ActivePatch
		if !request.Bind(c, &in) {
			return
		}
		if in.IsActive == nil {
			response.BadRequest(c, "is_active is required", nil)
			return
		}
		row, e := h.uc.SetActive(c.Request.Context(), request.Actor(c), c.Param("id"), *in.IsActive)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "User updated", row)
	})
	// @Summary Berikan role global
	// @Tags IAM
	// @Description Superadmin.
	// @Security BearerAuth
	// @Accept json
	// @Param request body RoleGrant true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/users/{id}/roles [post]
	g.POST("/users/:id/roles", func(c *gin.Context) {
		var in RoleGrant
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.Grant(c.Request.Context(), request.Actor(c), "", c.Param("id"), in.Role); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	// @Summary Penugasan arena
	// @Tags IAM
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/assignments [get]
	g.GET("/arenas/:arena/assignments", func(c *gin.Context) {
		rows, e := h.uc.Assignments(c.Request.Context(), request.Actor(c), c.Param("arena"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Assignments retrieved", rows)
	})
	// @Summary Tugaskan admin/operator/juri
	// @Tags IAM
	// @Description Admin arena boleh menugaskan operator/juri; admin_arena hanya oleh superadmin.
	// @Security BearerAuth
	// @Accept json
	// @Param request body ArenaAssignment true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/assignments [post]
	g.POST("/arenas/:arena/assignments", func(c *gin.Context) {
		var in ArenaAssignment
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.Grant(c.Request.Context(), request.Actor(c), c.Param("arena"), in.UserID, in.Role); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	// @Summary Hapus penugasan arena
	// @Tags IAM
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param assignment path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/assignments/{assignment} [delete]
	g.DELETE("/arenas/:arena/assignments/:assignment", func(c *gin.Context) {
		if e := h.uc.Revoke(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("assignment")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
}
