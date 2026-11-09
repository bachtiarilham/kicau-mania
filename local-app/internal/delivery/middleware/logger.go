package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/rest/response"

	"kicau.local/edge/internal/infra/logger"
)

// LoggerAndTraceMiddleware correlates HTTP and application logs without logging payloads.
func LoggerAndTraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" || len(requestID) > 128 || strings.IndexFunc(requestID, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.')
		}) >= 0 {
			var value [16]byte
			if _, err := rand.Read(value[:]); err != nil {
				c.Abort()
				response.WriteError(c, apperror.ErrUnavailable)
				return
			}
			requestID = hex.EncodeToString(value[:])
		}
		c.Header("X-Request-ID", requestID)
		c.Set("requestId", requestID)
		c.Set("traceID", requestID)
		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)
		started := time.Now()
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = "/unmatched"
		}
		fields := []zap.Field{zap.String("method", c.Request.Method), zap.String("path", path),
			zap.Int("status", c.Writer.Status()), zap.Duration("latency", time.Since(started)), zap.String("ip", c.ClientIP())}
		if kind, ok := c.Get("application_error"); ok {
			if value, ok := kind.(string); ok {
				fields = append(fields, zap.String("error_kind", value))
			}
		}
		log := logger.FromContext(ctx)
		switch {
		case c.Writer.Status() >= 500:
			log.Error("HTTP Request Failed", fields...)
		case c.Writer.Status() >= 400:
			log.Warn("HTTP Request Client Error", fields...)
		default:
			log.Info("HTTP Request", fields...)
		}
	}
}
