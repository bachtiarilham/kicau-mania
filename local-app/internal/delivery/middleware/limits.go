package middleware

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func RequestLimits() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2*1024*1024)
		c.Next()
	}
}
