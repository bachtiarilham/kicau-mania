package http

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/config"
	audioDelivery "kicau.local/edge/internal/audio/delivery"
	"kicau.local/edge/internal/delivery/http/middleware"
	savingDelivery "kicau.local/edge/internal/saving/delivery"
	streamingDelivery "kicau.local/edge/internal/streaming/delivery"
	videoDelivery "kicau.local/edge/internal/video/delivery"
)

type Handlers struct {
	Video     *videoDelivery.Handler
	Audio     *audioDelivery.Handler
	Streaming *streamingDelivery.Handler
	Saving    *savingDelivery.Handler
}

func Router(c config.Config, handlers Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		slog.Error("HTTP panic recovered")
		c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})
	}))
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 65536)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	})
	r.GET("/healthz", health)
	g := r.Group("/api/v1", middleware.Auth(c.Token, c.Origins))
	g.OPTIONS("/*path", preflight)
	videoDelivery.RegisterRoutes(g, handlers.Video)
	audioDelivery.RegisterRoutes(g, handlers.Audio)
	streamingDelivery.RegisterRoutes(g, handlers.Streaming)
	savingDelivery.RegisterRoutes(g, handlers.Saving)
	return r
}
