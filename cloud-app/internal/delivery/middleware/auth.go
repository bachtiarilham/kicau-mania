package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UserVerifier interface {
	AuthenticateUser(context.Context, string) (model.UserIdentity, error)
}
type EngineVerifier interface {
	AuthenticateEngine(context.Context, string) (model.EngineIdentity, error)
}
type userKey struct{}
type engineKey struct{}

func User(ctx context.Context) (model.UserIdentity, bool) {
	v, ok := ctx.Value(userKey{}).(model.UserIdentity)
	return v, ok
}
func Engine(ctx context.Context) (model.EngineIdentity, bool) {
	v, ok := ctx.Value(engineKey{}).(model.EngineIdentity)
	return v, ok
}
func Bearer(c *gin.Context) (string, bool) {
	if len(c.Request.Header.Values("Authorization")) != 1 {
		return "", false
	}
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 8192 {
		return "", false
	}
	return parts[1], true
}
func RequireUser(v UserVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := Bearer(c)
		if !ok {
			response.WriteError(c, apperror.ErrUnauthorized)
			c.Abort()
			return
		}
		if v == nil {
			response.WriteError(c, apperror.ErrUnavailable)
			c.Abort()
			return
		}
		id, e := v.AuthenticateUser(c.Request.Context(), token)
		if e != nil {
			response.WriteError(c, e)
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), userKey{}, id))
		c.Next()
	}
}
func RequireEngine(v EngineVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := Bearer(c)
		if !ok {
			response.WriteError(c, apperror.ErrUnauthorized)
			c.Abort()
			return
		}
		if v == nil {
			response.WriteError(c, apperror.ErrUnavailable)
			c.Abort()
			return
		}
		id, e := v.AuthenticateEngine(c.Request.Context(), token)
		if e != nil {
			response.WriteError(c, e)
			c.Abort()
			return
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), engineKey{}, id))
		c.Next()
	}
}
