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
	// @Summary Ronde yang ditugaskan
	// @Tags Judging
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
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
	// @Router /api/v1/judging/rounds [get]
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
	// @Summary Peserta terkonfirmasi menurut gantangan
	// @Tags Judging
	// @Description Juri arena dan anggota roster ronde.
	// @Security BearerAuth
	// @Produce json
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/judging/rounds/{round}/participants [get]
	g.GET("/judging/rounds/:round/participants", func(c *gin.Context) {
		rows, e := h.uc.Participants(c.Request.Context(), request.Actor(c), c.Param("round"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Participants retrieved", rows)
	})
	path := "/judging/rounds/:round/participants/:item/assessment"
	// @Summary Nilai sendiri, versi dan kriteria
	// @Tags Judging
	// @Description Belum submit: version 0, scores kosong. item adalah registration_item ID.
	// @Security BearerAuth
	// @Produce json
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param item path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/judging/rounds/{round}/participants/{item}/assessment [get]
	g.GET(path, func(c *gin.Context) {
		row, e := h.uc.Assessment(c.Request.Context(), request.Actor(c), c.Param("round"), c.Param("item"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Assessment retrieved", row)
	})
	// @Summary History revisi milik juri
	// @Tags Judging
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param item path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/judging/rounds/{round}/participants/{item}/assessment/history [get]
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
	// @Summary Submit seluruh kriteria
	// @Tags Judging
	// @Description Juri roster, fase JUDGING, expected_version 0. Seluruh kriteria tepat sekali, integer 1–10. Retry ID/payload identik; konflik 409.
	// @Security BearerAuth
	// @Accept json
	// @Param request body input.Submit true "JSON body"
	// @Produce json
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param item path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/judging/rounds/{round}/participants/{item}/assessment [post]
	g.POST(path, save(false))
	// @Summary Revisi seluruh nilai
	// @Tags Judging
	// @Description Fase JUDGING; expected_version terakhir. operation_id baru untuk revisi baru; retry memakai ID/payload identik. History tersimpan.
	// @Security BearerAuth
	// @Accept json
	// @Param request body input.Submit true "JSON body"
	// @Produce json
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param item path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/judging/rounds/{round}/participants/{item}/assessment [put]
	g.PUT(path, save(true))
}
