package delivery

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/media/recording-sessions", handler.startSession)
	group.POST("/media/recording-sessions/:id/stop", handler.stopSession)
	group.GET("/recordings", handler.recordings)
}
