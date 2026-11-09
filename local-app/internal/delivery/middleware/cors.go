package middleware

import (
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/rest/response"
	"net/url"
)

func originAllowed(c *gin.Context, origin string, origins []string) bool {
	for _, v := range origins {
		if v == origin {
			return true
		}
	}
	u, err := url.Parse(origin)
	return err == nil && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "" && (u.Scheme == "http" || u.Scheme == "https") && u.Host == c.Request.Host
}

// CORS checks origins and owns CORS headers for REST, WHEP, and WebSocket handshakes.
// Requests without Origin remain usable by native clients; authentication is separate.
func CORS(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin != "" {
			allowed := originAllowed(c, origin, origins)
			if !allowed {
				c.Abort()
				response.WriteError(c, apperror.Public(apperror.ErrForbidden, "origin denied"))
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, ETag, ID, Accept-Patch, Link, Location")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID, If-Match")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
