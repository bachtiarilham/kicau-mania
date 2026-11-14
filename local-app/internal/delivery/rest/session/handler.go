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
	Current(context.Context) (*model.Session, error)
	Reset(context.Context) error
}
type Handler struct{ uc SessionUseCase }

func New(uc SessionUseCase) *Handler { return &Handler{uc: uc} }
func RegisterRoutes(g *gin.RouterGroup, h *Handler) {
	g.POST("/media/sessions", h.start)
	g.POST("/media/sessions/reset", h.reset)
	g.POST("/media/sessions/:id/stop", h.stop)
	g.GET("/media/sessions/current", h.current)
	g.GET("/media/sessions/:id", h.get)
}

// start godoc
// @Summary Mulai seluruh media sesi
// @Description OperatorToken atau mode LAN dari localhost dengan LocalControl. Satu sesi nonterminal per engine. Start mengaktifkan live, recording, PCM/metrik dan restream pilihan. Stop diterima asinkron; poll sampai STOPPED/INTERRUPTED/FAILED, termasuk tahap FINALIZING. Upload tetap berjalan setelah Stop. HTTP 202 bukan bukti semua channel sudah RUNNING atau STOPPED.
// @Tags Media sessions
// @Accept json
// @Produce json
// @Security OperatorToken
// @Security LocalControl
// @Param request body model.StartRequest true "ID sesi, channel dan target restream terkonfigurasi"
// @Success 202 {object} response.Response{data=model.Session}
// @Failure 400,401,403,404,409,503 {object} response.Response
// @Router /api/v1/media/sessions [post]
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

// stop godoc
// @Summary Stop media dan mulai finalisasi rekaman
// @Description OperatorToken atau mode LAN dari localhost dengan LocalControl. Satu sesi nonterminal per engine. Start mengaktifkan live, recording, PCM/metrik dan restream pilihan. Stop diterima asinkron; poll sampai STOPPED/INTERRUPTED/FAILED, termasuk tahap FINALIZING. Upload tetap berjalan setelah Stop. HTTP 202 bukan bukti semua channel sudah RUNNING atau STOPPED.
// @Tags Media sessions
// @Accept json
// @Produce json
// @Security OperatorToken
// @Security LocalControl
// @Param id path string true "ID sesi"
// @Success 202 {object} response.Response{data=model.Session}
// @Failure 400,401,403,404,409,503 {object} response.Response
// @Router /api/v1/media/sessions/{id}/stop [post]
func (h *Handler) stop(c *gin.Context) {
	s, err := h.uc.StopSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Accepted(c, "Media session stop accepted", s)
}

// get godoc
// @Summary Baca status sesi dan masing-masing channel
// @Description OperatorToken atau mode LAN dari localhost dengan LocalControl. Satu sesi nonterminal per engine. Start mengaktifkan live, recording, PCM/metrik dan restream pilihan. Stop diterima asinkron; poll sampai STOPPED/INTERRUPTED/FAILED, termasuk tahap FINALIZING. Upload tetap berjalan setelah Stop. HTTP 202 bukan bukti semua channel sudah RUNNING atau STOPPED.
// @Tags Media sessions
// @Accept json
// @Produce json
// @Security OperatorToken
// @Security LocalControl
// @Param id path string true "ID sesi"
// @Success 200 {object} response.Response{data=model.Session}
// @Failure 400,401,403,404,409,503 {object} response.Response
// @Router /api/v1/media/sessions/{id} [get]
func (h *Handler) get(c *gin.Context) {
	s, err := h.uc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Media session retrieved", s)
}

// current godoc
// @Summary Baca status sesi media yang sedang aktif atau terakhir
// @Tags Media sessions
// @Produce json
// @Security LocalControl
// @Success 200 {object} response.Response{data=model.Session}
// @Router /api/v1/media/sessions/current [get]
func (h *Handler) current(c *gin.Context) {
	s, err := h.uc.Current(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	if s == nil {
		response.Success(c, "No session active", nil)
		return
	}
	response.Success(c, "Current media session", s)
}

// reset godoc
// @Summary Reset riwayat sesi media dan manifest
// @Tags Media sessions
// @Produce json
// @Security LocalControl
// @Success 200 {object} response.Response
// @Router /api/v1/media/sessions/reset [post]
func (h *Handler) reset(c *gin.Context) {
	if err := h.uc.Reset(c.Request.Context()); err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Media sessions reset", nil)
}
