package delivery

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/camera/model"
	"kicau.local/edge/internal/delivery/rest/response"
)

type CameraUseCase interface {
	Channels() []model.PlaybackChannel
}

type ZoomUseCase interface {
	ListActiveChannels(context.Context) []model.ZoomChannel
	GetChannel(context.Context, string) (model.ZoomChannel, error)
	UpdateCalibration(context.Context, string, model.CropConfig, model.PerspectiveConfig) error
}

type ChannelResponse struct {
	ID    string `json:"id"`
	RTSP  string `json:"rtsp_url"`
	Audio string `json:"audio_path"`
}

type CalibrationRequest struct {
	Crop        model.CropConfig        `json:"crop" binding:"required"`
	Perspective model.PerspectiveConfig `json:"perspective"`
}

type Handler struct {
	camera CameraUseCase
	zoom   ZoomUseCase
}

func NewHandler(camera CameraUseCase, zoom ZoomUseCase) *Handler {
	return &Handler{
		camera: camera,
		zoom:   zoom,
	}
}

func (h *Handler) Routes(readGroup, opGroup *gin.RouterGroup) {
	if readGroup != nil && h.camera != nil {
		readGroup.GET("/channels", h.Channels)
	}
	if opGroup != nil && h.zoom != nil {
		opGroup.GET("/zoom/channels", h.ListZoomChannels)
		opGroup.GET("/zoom/channels/:id", h.GetZoomChannel)
		opGroup.PUT("/zoom/channels/:id/calibration", h.UpdateCalibration)
	}
}

func (h *Handler) Channels(c *gin.Context) {
	records := h.camera.Channels()
	out := make([]ChannelResponse, 0, len(records))
	for _, ch := range records {
		out = append(out, ChannelResponse{
			ID:    ch.ID,
			RTSP:  ch.RTSP,
			Audio: "/api/v1/channels/" + ch.ID + "/audio",
		})
	}
	response.Success(c, "Channels retrieved", out)
}

func (h *Handler) ListZoomChannels(c *gin.Context) {
	channels := h.zoom.ListActiveChannels(c.Request.Context())
	response.Success(c, "Active zoom channels retrieved", channels)
}

func (h *Handler) GetZoomChannel(c *gin.Context) {
	id := c.Param("id")
	ch, err := h.zoom.GetChannel(c.Request.Context(), id)
	if err != nil {
		response.JSON(c, http.StatusNotFound, false, "Zoom channel not found", nil, nil)
		return
	}
	response.Success(c, "Zoom channel retrieved", ch)
}

func (h *Handler) UpdateCalibration(c *gin.Context) {
	id := c.Param("id")
	var req CalibrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid calibration payload", err.Error())
		return
	}

	if err := h.zoom.UpdateCalibration(c.Request.Context(), id, req.Crop, req.Perspective); err != nil {
		response.BadRequest(c, err.Error(), nil)
		return
	}

	response.Success(c, "Calibration updated successfully", nil)
}
