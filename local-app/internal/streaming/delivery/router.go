package delivery

import "github.com/gin-gonic/gin"

func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.GET("/restreams", handler.list)
	group.POST("/restreams", handler.start)
	group.DELETE("/restreams/:target", handler.stop)
}
