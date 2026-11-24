package delivery

import (
	"context"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/streaming/model"
)

type RestreamUseCase interface {
	Restreams() []model.RestreamView
}

type AudioUseCase interface {
	SubscribeAudio(string) (<-chan []byte, func(), error)
	SubscribeMetrics() (<-chan []byte, func(), error)
	SubscribePreviewMetrics() (<-chan []byte, func(), error)
}

type Handler struct {
	restream RestreamUseCase
	audio    AudioUseCase
	stream   *websocketStream
}

func NewHandler(restream RestreamUseCase, audio AudioUseCase, origins []string) *Handler {
	return &Handler{
		restream: restream,
		audio:    audio,
		stream:   newWebsocketStream(origins),
	}
}

func (h *Handler) Routes(readGroup, opGroup *gin.RouterGroup) {
	if readGroup != nil && h.audio != nil {
		readGroup.GET("/channels/:id/audio", h.Audio)
		readGroup.GET("/metrics", h.Metrics)
	}
	if opGroup != nil && h.restream != nil {
		opGroup.GET("/restreams", h.Restreams)
	}
}

func (h *Handler) Audio(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.audio.SubscribeAudio(c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.BinaryMessage, generation)
}

func (h *Handler) Metrics(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.audio.SubscribeMetrics()
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.TextMessage, generation)
}

func (h *Handler) PreviewMetrics(c *gin.Context) {
	generation := h.stream.generation()
	messages, cancel, err := h.audio.SubscribePreviewMetrics()
	if err != nil {
		response.WriteError(c, err)
		return
	}
	defer cancel()
	h.stream.serve(c.Writer, c.Request, messages, gorillaws.TextMessage, generation)
}

func (h *Handler) Restreams(c *gin.Context) {
	response.Success(c, "Restreams retrieved", h.restream.Restreams())
}

func (h *Handler) CloseSessions(ctx context.Context) error {
	return h.stream.CloseSessions(ctx)
}

func (h *Handler) Close() {
	h.stream.Close()
}
