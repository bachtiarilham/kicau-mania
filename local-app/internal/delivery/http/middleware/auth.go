package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
)

func Auth(token string, origins []string) gin.HandlerFunc {
	expected := sha256.Sum256([]byte(token))
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
		provided := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if provided == "" && c.Request.Method == "GET" {
			for _, p := range strings.Split(c.GetHeader("Sec-WebSocket-Protocol"), ",") {
				if encoded, ok := strings.CutPrefix(strings.TrimSpace(p), "edge-token."); ok {
					b, e := base64.RawURLEncoding.DecodeString(encoded)
					if e == nil {
						provided = string(b)
					}
				}
			}
		}
		actual := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("actor", "local-operator")
		c.Next()
	}
}
