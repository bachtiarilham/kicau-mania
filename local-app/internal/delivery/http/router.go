package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/config"
	arenaDelivery "kicau.local/edge/internal/core/arena/delivery"
	"kicau.local/edge/internal/delivery/http/middleware"
	"kicau.local/edge/internal/infra/logger"
	audioDelivery "kicau.local/edge/internal/mediaengine/audio/delivery"
	savingDelivery "kicau.local/edge/internal/mediaengine/saving/delivery"
	streamingDelivery "kicau.local/edge/internal/mediaengine/streaming/delivery"
	videoDelivery "kicau.local/edge/internal/mediaengine/video/delivery"
)

type Handlers struct {
	Arena     *arenaDelivery.Handler
	Video     *videoDelivery.Handler
	Audio     *audioDelivery.Handler
	Streaming *streamingDelivery.Handler
	Saving    *savingDelivery.Handler
}

func Router(c config.Config, handlers Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(middleware.LoggerAndTraceMiddleware())
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		logger.FromContext(c.Request.Context()).Error("HTTP panic recovered")
		c.AbortWithStatusJSON(500, gin.H{"error": "internal server error"})
	}))
	r.Use(func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
		c.Header("X-Content-Type-Options", "nosniff")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	})
	r.GET("/healthz", health)
	r.GET("/", func(c *gin.Context) { c.Redirect(307, "/arena/") })
	if handlers.Arena != nil {
		arenaDelivery.RegisterRoutes(r, handlers.Arena, middleware.Auth(c.Token, c.Origins), middleware.Origin(c.Origins))
	}
	g := r.Group("/api/v1", middleware.Auth(c.Token, c.Origins))
	g.OPTIONS("/*path", preflight)
	videoDelivery.RegisterRoutes(g, handlers.Video)
	audioDelivery.RegisterRoutes(g, handlers.Audio)
	streamingDelivery.RegisterRoutes(g, handlers.Streaming)
	savingDelivery.RegisterRoutes(g, handlers.Saving)
	return r
}
