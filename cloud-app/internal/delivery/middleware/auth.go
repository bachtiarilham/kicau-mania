// Package middleware handles HTTP concerns; authentication decisions belong to injected usecases.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/delivery/rest/response"
)

// User and engine credentials must be verified by different application ports.
type UserVerifier interface {
	AuthenticateUser(context.Context, string) (model.UserIdentity, error)
}
type EngineVerifier interface {
	AuthenticateEngine(context.Context, string) (model.EngineIdentity, error)
}
type userKey struct{}
type engineKey struct{}

func User(ctx context.Context) (model.UserIdentity, bool) {
	value, ok := ctx.Value(userKey{}).(model.UserIdentity)
	return value, ok
}
func Engine(ctx context.Context) (model.EngineIdentity, bool) {
	value, ok := ctx.Value(engineKey{}).(model.EngineIdentity)
	return value, ok
}

func bearer(r *http.Request) (string, bool) {
	if len(r.Header.Values("Authorization")) != 1 {
		return "", false
	}
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 8192 {
		return "", false
	}
	return parts[1], true
}

// RequireUser is a boundary for future user routes, not a login implementation.
func RequireUser(verifier UserVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearer(r)
			if !ok {
				response.WriteError(w, apperror.ErrUnauthorized)
				return
			}
			if verifier == nil {
				response.WriteError(w, apperror.ErrUnavailable)
				return
			}
			identity, err := verifier.AuthenticateUser(r.Context(), token)
			if err != nil {
				response.WriteError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, identity)))
		})
	}
}

func RequireEngine(verifier EngineVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearer(r)
			if !ok {
				response.WriteError(w, apperror.ErrUnauthorized)
				return
			}
			if verifier == nil {
				response.WriteError(w, apperror.ErrUnavailable)
				return
			}
			identity, err := verifier.AuthenticateEngine(r.Context(), token)
			if err != nil {
				response.WriteError(w, err)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), engineKey{}, identity)))
		})
	}
}
