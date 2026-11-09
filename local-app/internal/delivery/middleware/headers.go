package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders applies shared policy to successes, errors, and handshakes.
// Content-Type and protocol-specific headers belong to the response transport.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
