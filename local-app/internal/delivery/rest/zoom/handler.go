package zoom

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/zoom/model"
)

type ZoomUseCase interface {
	ListActiveChannels(context.Context) []model.ZoomChannel
	GetChannel(context.Context, string) (model.ZoomChannel, error)
	UpdateCalibration(context.Context, string, model.CropConfig, model.PerspectiveConfig) error
}

type Handler struct {
	uc ZoomUseCase
}

func New(uc ZoomUseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/zoom/channels", h.listChannels)
	g.GET("/zoom/channels/:id", h.getChannel)
	g.PUT("/zoom/channels/:id/calibration", h.updateCalibration)
}

func (h *Handler) listChannels(c *gin.Context) {
	channels := h.uc.ListActiveChannels(c.Request.Context())
	response.Success(c, "Active zoom channels retrieved", channels)
}

func (h *Handler) getChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := h.uc.GetChannel(c.Request.Context(), id)
	if err != nil {
		response.JSON(c, http.StatusNotFound, false, "Zoom channel not found", nil, nil)
		return
	}
	response.Success(c, "Zoom channel retrieved", ch)
}

type CalibrationRequest struct {
	Crop        model.CropConfig        `json:"crop" binding:"required"`
	Perspective model.PerspectiveConfig `json:"perspective"`
}

func (h *Handler) updateCalibration(c *gin.Context) {
	id := c.Param("id")
	var req CalibrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid calibration payload", err.Error())
		return
	}

	if err := h.uc.UpdateCalibration(c.Request.Context(), id, req.Crop, req.Perspective); err != nil {
		response.BadRequest(c, err.Error(), nil)
		return
	}

	response.Success(c, "Calibration updated successfully", nil)
}
