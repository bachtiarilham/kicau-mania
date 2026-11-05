package middleware

import (
	"github.com/gin-gonic/gin"
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

// Origin protects cookie-authenticated state changes as well as cross-origin APIs.
// Requests without Origin remain usable by native clients; authentication is separate.
func Origin(origins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			allowed := originAllowed(c, origin, origins)
			if !allowed {
				c.AbortWithStatusJSON(403, gin.H{"error": "origin denied"})
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		}
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
