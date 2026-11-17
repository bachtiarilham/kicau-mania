package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/channel/model"
)

// ChannelUseCase provides the channel data needed by this handler.
type ChannelUseCase interface {
	Channels() []model.PlaybackChannel
}

// ChannelResponse is the REST representation of a playable channel.
type ChannelResponse struct {
	ID    string `json:"id"`
	RTSP  string `json:"rtsp_url"`
	Audio string `json:"audio_path"`
}

type Handler struct {
	uc ChannelUseCase
}

func New(uc ChannelUseCase) *Handler {
	return &Handler{uc}
}

// channels godoc
// @Summary Daftar channel dan alamat RTSP/audio
// @Description Mode LAN: subnet pembaca tanpa token. Mode cloud: OperatorToken. Katalog tidak otomatis memulai stream; channel harus masuk sesi aktif untuk playback.
// @Tags Channels
// @Security OperatorToken
// @Produce json
// @Success 200 {object} response.Response{data=[]ChannelResponse}
// @Failure 401,403 {object} response.Response
// @Router /api/v1/channels [get]
func (h *Handler) channels(c *gin.Context) {
	records := h.uc.Channels()
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
