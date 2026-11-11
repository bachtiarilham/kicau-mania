package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/streaming/model"
)

// RestreamUseCase contains only the restream operations exposed by this handler.
type RestreamUseCase interface {
	Restreams() []model.RestreamView
}

type Handler struct{ uc RestreamUseCase }

func New(uc RestreamUseCase) *Handler {
	return &Handler{uc}
}

// list godoc
// @Summary Daftar status restream
// @Description Kontrol restream dilakukan bersama sesi media, bukan melalui endpoint start/stop terpisah.
// @Tags Restreams
// @Produce json
// @Security OperatorToken
// @Security LocalControl
// @Success 200 {object} response.Response{data=[]model.RestreamView}
// @Failure 401,403 {object} response.Response
// @Router /api/v1/restreams [get]
func (h *Handler) list(c *gin.Context) {
	response.Success(c, "Restreams retrieved", h.uc.Restreams())
}
