// Package middleware handles shared HTTP headers, request limits, and authentication.
package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/rest/response"
)

func Auth(token string) gin.HandlerFunc {
	expected := sha256.Sum256([]byte(token))
	return func(c *gin.Context) {
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
			c.Abort()
			response.WriteError(c, apperror.ErrUnauthorized)
			return
		}
		c.Set("actor", "local-operator")
		c.Next()
	}
}
