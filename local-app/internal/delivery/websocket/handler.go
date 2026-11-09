// Package websocket delivers audio and metrics through managed WebSocket connections.
package websocket

import (
	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"

	"kicau.local/edge/internal/delivery/rest/response"
)

type AudioUseCase interface {
	SubscribeAudio(string) (<-chan []byte, func(), error)
	SubscribeMetrics() (<-chan []byte, func())
}

type Handler struct {
	uc     AudioUseCase
	stream *websocketStream
}

func New(uc AudioUseCase, origins []string) *Handler {
	return &Handler{uc: uc, stream: newWebsocketStream(origins)}
}

func (h *Handler) audio(c *gin.Context) {
	messages, cancel, err := h.uc.SubscribeAudio(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.BinaryMessage)
}

func (h *Handler) metrics(c *gin.Context) {
	messages, cancel := h.uc.SubscribeMetrics()
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.TextMessage)
}

func (h *Handler) Close() { h.stream.Close() }
