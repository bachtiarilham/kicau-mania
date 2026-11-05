package delivery

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"kicau.local/edge/internal/delivery/http/response"
	"kicau.local/edge/internal/mediaengine/audio/usecase"
)

type Handler struct {
	uc     *usecase.UseCase
	stream *websocketStream
}

func New(uc *usecase.UseCase, origins []string) *Handler {
	return &Handler{uc: uc, stream: newWebsocketStream(origins)}
}

func (h *Handler) audio(c *gin.Context) {
	messages, cancel, err := h.uc.SubscribeAudio(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, websocket.BinaryMessage)
}

func (h *Handler) metrics(c *gin.Context) {
	messages, cancel := h.uc.SubscribeMetrics()
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, websocket.TextMessage)
}

func (h *Handler) Close() { h.stream.Close() }
