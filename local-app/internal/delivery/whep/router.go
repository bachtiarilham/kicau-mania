package whep

import "github.com/gin-gonic/gin"

// RegisterRoutes receives the playback group after its origin middleware is installed.
func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/:channel/whep", handler.Serve)
	group.PATCH("/:channel/whep/:session", handler.Serve)
	group.DELETE("/:channel/whep/:session", handler.Serve)
}
