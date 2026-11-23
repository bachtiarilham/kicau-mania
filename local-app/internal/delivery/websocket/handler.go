// Package websocket delivers audio and metrics through managed WebSocket connections.
package websocket

import (
	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"

	"kicau.local/edge/internal/delivery/rest/response"
)

type AudioUseCase interface {
	SubscribeAudio(string) (<-chan []byte, func(), error)
	SubscribeMetrics() (<-chan []byte, func(), error)
	SubscribePreviewMetrics() (<-chan []byte, func(), error)
}

type Handler struct {
	uc     AudioUseCase
	stream *websocketStream
}

func New(uc AudioUseCase, origins []string) *Handler {
	return &Handler{uc: uc, stream: newWebsocketStream(origins)}
}

// audio godoc
// @Summary Subscribe audio PCM channel
// @Description Upgrade WebSocket (ws/wss), subprotocol edge-v1. Swagger Try it out tidak dapat membuka WebSocket. Mode LAN: subnet pembaca tanpa token; mode cloud: Bearer API_TOKEN atau subprotocol edge-v1 bersama edge-token.BASE64URL_TOKEN (tanpa padding). Origin tetap diperiksa. Frame binary PCM signed 16-bit little-endian, mono 16 kHz. Channel harus dalam sesi aktif dan memiliki audio. Tidak membuat audio senyap untuk sumber video-only.
// @Tags WebSocket
// @Security OperatorToken
// @Param Sec-WebSocket-Protocol header string true "edge-v1 (ditambah edge-token.BASE64URL_TOKEN bila mode cloud)"
// @Param id path string true "ID channel"
// @Success 101 "Switching Protocols, frame WebSocket bukan envelope REST"
// @Failure 400,401,403,404,409,503 {object} response.Response
// @Router /api/v1/channels/{id}/audio [get]
func (h *Handler) audio(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.uc.SubscribeAudio(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.BinaryMessage, generation)
}

// metrics godoc
// @Summary Subscribe metrik audio
// @Description Upgrade WebSocket (ws/wss), subprotocol edge-v1. Swagger Try it out tidak dapat membuka WebSocket. Mode LAN: subnet pembaca tanpa token; mode cloud: Bearer API_TOKEN atau subprotocol edge-v1 bersama edge-token.BASE64URL_TOKEN (tanpa padding). Origin tetap diperiksa. Frame text JSON: channel_id, at, peak_dbfs, rms_dbfs, active_ms, window_ms. Distribusi metrik hanya untuk sumber audio sesi aktif; bukan response JSON REST.
// @Tags WebSocket
// @Security OperatorToken
// @Param Sec-WebSocket-Protocol header string true "edge-v1 (ditambah edge-token.BASE64URL_TOKEN bila mode cloud)"
// @Success 101 "Switching Protocols, frame WebSocket bukan envelope REST"
// @Failure 400,401,403,404,409,503 {object} response.Response
// @Router /api/v1/metrics [get]
func (h *Handler) metrics(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.uc.SubscribeMetrics()
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.TextMessage, generation)
}

func (h *Handler) PreviewMetrics(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.uc.SubscribePreviewMetrics()
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.TextMessage, generation)
}

func (h *Handler) Close() { h.stream.Close() }
