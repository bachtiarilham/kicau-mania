package delivery

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/channels/:id/audio", handler.audio)
	group.GET("/metrics", handler.metrics)
}
