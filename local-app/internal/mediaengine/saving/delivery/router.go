package delivery

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/media/recording-sessions", handler.startRound)
	group.POST("/media/recording-sessions/:id/stop", handler.stopRound)
	group.POST("/rounds", handler.startRound)
	group.POST("/rounds/:id/stop", handler.stopRound)
	group.GET("/recordings", handler.recordings)
}
