// Package middleware owns transport limits, tracing, CORS and authentication extraction.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/response"
	"kicau.cloud/control/internal/infra/logger"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func AccessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			var b [16]byte
			if _, e := rand.Read(b[:]); e != nil {
				response.WriteError(c, e)
				c.Abort()
				return
			}
			id = hex.EncodeToString(b[:])
		}
		c.Header("X-Request-ID", id)
		c.Request = c.Request.WithContext(logger.WithRequestID(c.Request.Context(), id))
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		logger.FromContext(c.Request.Context()).Info("HTTP request", zap.String("method", c.Request.Method), zap.String("route", route), zap.Int("status", c.Writer.Status()), zap.Duration("latency", time.Since(start)))
	}
}
func Recover() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recover() != nil {
				logger.FromContext(c.Request.Context()).Error("HTTP panic recovered")
				c.Abort()
				if !c.Writer.Written() {
					response.WriteError(c, nil)
				}
			}
		}()
		c.Next()
	}
}
func Limits(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := int64(2 * 1024 * 1024)
		if strings.HasSuffix(c.Request.URL.Path, "/proof") {
			limit = 6 * 1024 * 1024
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
func CORS(origins []string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(c *gin.Context) {
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if origin != "" {
			if !allowed[origin] {
				response.WriteError(c, apperror.ErrForbidden)
				c.Abort()
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID")
			if c.Request.Method == "OPTIONS" {
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				c.Status(204)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func CanonicalParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		for i := range c.Params {
			if model.UUID(c.Params[i].Value) {
				c.Params[i].Value = strings.ToLower(c.Params[i].Value)
			}
		}
		q := c.Request.URL.Query()
		if arena := q.Get("arena_id"); model.UUID(arena) {
			q.Set("arena_id", strings.ToLower(arena))
			c.Request.URL.RawQuery = q.Encode()
		}
		c.Next()
	}
}
