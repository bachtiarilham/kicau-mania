// Package delivery assembles protocol routes and shared middleware.
package delivery

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/middleware"
	"kicau.local/edge/internal/delivery/rest"
	engineDelivery "kicau.local/edge/internal/delivery/rest/engine"
	"kicau.local/edge/internal/delivery/rest/response"
	savingDelivery "kicau.local/edge/internal/delivery/rest/saving"
	streamingDelivery "kicau.local/edge/internal/delivery/rest/streaming"
	videoDelivery "kicau.local/edge/internal/delivery/rest/video"
	"kicau.local/edge/internal/delivery/websocket"
	"kicau.local/edge/internal/delivery/whep"
	"kicau.local/edge/internal/infra/logger"
)

type Handlers struct {
	Engine    *engineDelivery.Handler
	WHEP      *whep.Handler
	Video     *videoDelivery.Handler
	WebSocket *websocket.Handler
	Streaming *streamingDelivery.Handler
	Saving    *savingDelivery.Handler
}

func Router(c config.Config, handlers Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.HandleMethodNotAllowed = true
	_ = r.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.LoggerAndTraceMiddleware())
	r.Use(gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, _ any) {
		logger.FromContext(c.Request.Context()).Error("HTTP panic recovered")
		c.Abort()
		if !c.Writer.Written() {
			response.WriteError(c, nil)
		}
	}))
	r.Use(middleware.RequestLimits(), middleware.CORS(c.Origins))
	r.GET("/healthz", rest.Health)
	r.GET("/", func(ctx *gin.Context) {
		if c.FrontendOrigin != "" {
			ctx.Redirect(http.StatusTemporaryRedirect, c.FrontendOrigin)
			return
		}
		response.Success(ctx, "Media engine", gin.H{"service": "local-app media engine"})
	})
	if handlers.Engine != nil {
		r.GET("/readyz", handlers.Engine.Ready)
	}
	if handlers.WHEP != nil {
		playback := r.Group("/api/v1/playback")
		whep.RegisterRoutes(playback, handlers.WHEP)
	}
	g := r.Group("/api/v1", middleware.Auth(c.Token))
	r.OPTIONS("/api/v1/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	if handlers.Engine != nil {
		g.GET("/engine/status", handlers.Engine.Status)
	}
	videoDelivery.RegisterRoutes(g, handlers.Video)
	websocket.RegisterRoutes(g, handlers.WebSocket)
	streamingDelivery.RegisterRoutes(g, handlers.Streaming)
	savingDelivery.RegisterRoutes(g, handlers.Saving)
	r.NoRoute(func(c *gin.Context) { response.WriteError(c, apperror.ErrNotFound) })
	r.NoMethod(func(c *gin.Context) { response.JSON(c, 405, false, "method not allowed", nil, nil) })
	return r
}
