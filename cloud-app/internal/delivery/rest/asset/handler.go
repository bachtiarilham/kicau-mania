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
	// @Summary Cari burung melalui ring
	// @Tags Assets
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Param ring_number query string true "Nomor ring, tanpa kontak pemilik."
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds/lookup [get]
	g.GET("/birds/lookup", func(c *gin.Context) {
		row, e := h.uc.Lookup(c.Request.Context(), c.Query("ring_number"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Bird retrieved", row)
	})
	// @Summary Transfer kepemilikan burung
	// @Tags Assets
	// @Description Superadmin; pemilik dan history diperbarui atomik.
	// @Security BearerAuth
	// @Accept json
	// @Param request body BirdTransfer true "JSON body"
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
	// @Router /api/v1/birds/{id}/transfer [post]
	g.POST("/birds/:id/transfer", func(c *gin.Context) {
		var in BirdTransfer
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.Transfer(c.Request.Context(), request.Actor(c), c.Param("id"), in.ToOwnerID, in.Reason)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Owner transferred", row)
	})
}
