// Package delivery assembles protocol routes and shared middleware.
package delivery

import (
	"io"
	desktopDelivery "kicau.local/edge/internal/delivery/rest/desktop"
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/middleware"
	"kicau.local/edge/internal/delivery/rest"
	"kicau.local/edge/internal/delivery/rest/apidocs"
	channelDelivery "kicau.local/edge/internal/delivery/rest/channel"
	engineDelivery "kicau.local/edge/internal/delivery/rest/engine"
	"kicau.local/edge/internal/delivery/rest/response"
	savingDelivery "kicau.local/edge/internal/delivery/rest/saving"
	sessionDelivery "kicau.local/edge/internal/delivery/rest/session"
	setupDelivery "kicau.local/edge/internal/delivery/rest/setup"
	streamingDelivery "kicau.local/edge/internal/delivery/rest/streaming"
	zoomDelivery "kicau.local/edge/internal/delivery/rest/zoom"
	"kicau.local/edge/internal/delivery/websocket"
	"kicau.local/edge/internal/infra/logger"
)

type Handlers struct {
	Desktop   *desktopDelivery.Handler
	Setup     *setupDelivery.Handler
	Engine    *engineDelivery.Handler
	Channels  *channelDelivery.Handler
	WebSocket *websocket.Handler
	Streaming *streamingDelivery.Handler
	Saving    *savingDelivery.Handler
	Sessions  *sessionDelivery.Handler
	Zoom      *zoomDelivery.Handler
}

// Router godoc
// @Summary Identitas layanan atau redirect frontend
// @Description Jika FRONTEND_ORIGIN disetel, response berupa redirect 307 ke frontend; jika kosong, JSON identitas layanan.
// @Tags Health
// @Produce json
// @Success 200 {object} response.Response{data=object{service=string}}
// @Success 307 "Redirect ke FRONTEND_ORIGIN"
// @Router / [get]
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
	apidocs.Routes(r)
	r.GET("/healthz", rest.Health)
	r.GET("/", func(ctx *gin.Context) {
		if c.FrontendOrigin != "" {
			ctx.Redirect(http.StatusTemporaryRedirect, c.FrontendOrigin)
			return
		}
		response.Success(ctx, "Media engine", gin.H{"service": "local-app media engine"})
	})
	if c.AccessMode == "lan" {
		if handlers.Setup != nil {
			handlers.Setup.Routes(r.Group("/api/v1/setup", middleware.LocalControl(c.LocalControlToken, c.Origins)))
			r.GET("/api/v1/local/status", middleware.LANViewer(c.ReadCIDRs), handlers.Setup.Status)
			r.GET("/api/v1/local/guidance/:code", middleware.LANViewer(c.ReadCIDRs), handlers.Setup.Guidance)
		}
	}
	if handlers.Engine != nil {
		r.GET("/readyz", handlers.Engine.Ready)
	} else {
		r.GET("/readyz", func(ctx *gin.Context) { response.JSON(ctx, 503, false, "Engine setup required", nil, nil) })
	}
	g := r.Group("/api/v1", middleware.Operator(c.Token, c.LocalControlToken, c.AccessMode == "lan", c.Origins))
	if handlers.Desktop != nil && c.AccessMode == "lan" {
		handlers.Desktop.Routes(r.Group("/api/v1", middleware.LocalControl(c.LocalControlToken, c.Origins)))
	}
	r.OPTIONS("/api/v1/*path", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	if handlers.Engine != nil {
		g.GET("/engine/status", handlers.Engine.Status)
	}
	read := g
	if c.AccessMode == "lan" {
		read = r.Group("/api/v1", middleware.LANViewer(c.ReadCIDRs))
	}
	if handlers.Channels != nil {
		channelDelivery.RegisterRoutes(read, handlers.Channels)
	}
	if handlers.WebSocket != nil {
		websocket.RegisterRoutes(read, handlers.WebSocket)
		if c.AccessMode == "lan" {
			r.GET("/api/v1/local/preview-metrics", middleware.LocalControl(c.LocalControlToken, c.Origins), handlers.WebSocket.PreviewMetrics)
		}
	} else {
		read.GET("/metrics", func(c *gin.Context) { response.Success(c, "No metrics active", nil) })
	}
	if handlers.Streaming != nil {
		streamingDelivery.RegisterRoutes(g, handlers.Streaming)
	}
	if handlers.Saving != nil {
		savingDelivery.RegisterRoutes(g, handlers.Saving)
	}
	if handlers.Sessions != nil {
		sessionDelivery.RegisterRoutes(g, handlers.Sessions)
	} else {
		g.GET("/media/sessions/current", func(c *gin.Context) { response.Success(c, "No session active", nil) })
		g.GET("/media/sessions/:id", func(c *gin.Context) { response.Success(c, "No session active", nil) })
	}
	if handlers.Zoom != nil {
		handlers.Zoom.Routes(g)
	}
	r.NoRoute(func(c *gin.Context) { response.WriteError(c, apperror.ErrNotFound) })
	r.NoMethod(func(c *gin.Context) { response.JSON(c, 405, false, "method not allowed", nil, nil) })
	return r
}
