package http

import "github.com/gin-gonic/gin"

func health(c *gin.Context) {
	c.JSON(200, gin.H{"status": "UP"})
}

func preflight(c *gin.Context) {
	c.Status(204)
}
