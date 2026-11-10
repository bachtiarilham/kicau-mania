// Package auth parses authentication requests and owns HTTP-only cookie transport.
package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/apperror"
	input "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	Register(context.Context, input.Register) error
	Login(context.Context, input.Login) (input.Tokens, error)
	GoogleLogin(context.Context, input.GoogleLogin) (input.Tokens, error)
	LinkGoogle(context.Context, string, string) error
	RequestEmail(context.Context, string, string, string) error
	Consume(context.Context, string, string, string) error
	ChangePassword(context.Context, string, string, string) error
	Refresh(context.Context, string) (input.Tokens, error)
	Logout(context.Context, string, string) error
	Profile(context.Context, string) (model.Record, error)
	Sessions(context.Context, string) ([]model.Record, error)
	Revoke(context.Context, string, string) error
}
type Options struct {
	CookieName string
	Secure     bool
	Origins    []string
}
type Handler struct {
	uc   UseCase
	opts Options
}

func New(u UseCase, o Options) *Handler { return &Handler{u, o} }
func (h *Handler) cookie(c *gin.Context, value string, age int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.opts.CookieName, value, age, "/", "", h.opts.Secure, true)
}
func (h *Handler) tokens(c *gin.Context, t input.Tokens) {
	h.cookie(c, t.RefreshToken, 30*86400)
	response.Success(c, "Signed in", t)
}
func (h *Handler) Public(g *gin.RouterGroup) {
	g.POST("/register", func(c *gin.Context) {
		var in input.Register
		if !request.Bind(c, &in) {
			return
		}
		in.IP = c.ClientIP()
		if e := h.uc.Register(c.Request.Context(), in); e != nil {
			response.WriteError(c, e)
			return
		}
		response.Accepted(c, "Registration accepted; verify your email", nil)
	})
	g.POST("/login", func(c *gin.Context) {
		var in input.Login
		if !request.Bind(c, &in) {
			return
		}
		in.UserAgent, in.IP = c.Request.UserAgent(), c.ClientIP()
		out, e := h.uc.Login(c.Request.Context(), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		h.tokens(c, out)
	})
	g.POST("/google", func(c *gin.Context) {
		var in input.GoogleLogin
		if !request.Bind(c, &in) {
			return
		}
		in.UserAgent, in.IP = c.Request.UserAgent(), c.ClientIP()
		out, e := h.uc.GoogleLogin(c.Request.Context(), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		h.tokens(c, out)
	})
	for path, purpose := range map[string]string{"/verify-email/request": "VERIFY_EMAIL", "/forgot-password": "RESET_PASSWORD"} {
		purpose := purpose
		g.POST(path, func(c *gin.Context) {
			var in struct {
				Email string `json:"email"`
			}
			if !request.Bind(c, &in) {
				return
			}
			if e := h.uc.RequestEmail(c.Request.Context(), in.Email, purpose, c.ClientIP()); e != nil {
				response.WriteError(c, e)
				return
			}
			response.Accepted(c, "If eligible, account instructions will be sent", nil)
		})
	}
	for path, purpose := range map[string]string{"/verify-email": "VERIFY_EMAIL", "/reset-password": "RESET_PASSWORD"} {
		purpose := purpose
		g.POST(path, func(c *gin.Context) {
			var in struct {
				Token    string `json:"token"`
				Password string `json:"password"`
			}
			if !request.Bind(c, &in) {
				return
			}
			if e := h.uc.Consume(c.Request.Context(), in.Token, in.Password, purpose); e != nil {
				response.WriteError(c, e)
				return
			}
			response.Success(c, "Account updated", nil)
		})
	}
	g.POST("/refresh", func(c *gin.Context) {
		allowed := false
		for _, o := range h.opts.Origins {
			if c.GetHeader("Origin") == o {
				allowed = true
			}
		}
		if !allowed {
			response.WriteError(c, apperror.ErrForbidden)
			return
		}
		var in struct{}
		if !request.Bind(c, &in) {
			return
		}
		raw, e := c.Cookie(h.opts.CookieName)
		if e != nil {
			response.WriteError(c, apperror.ErrUnauthorized)
			return
		}
		out, e := h.uc.Refresh(c.Request.Context(), raw)
		if e != nil {
			h.cookie(c, "", -1)
			response.WriteError(c, e)
			return
		}
		h.tokens(c, out)
	})
}
func (h *Handler) Private(g *gin.RouterGroup) {
	g.GET("/me", func(c *gin.Context) {
		row, e := h.uc.Profile(c.Request.Context(), request.Actor(c))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Profile retrieved", row)
	})
	g.POST("/logout", func(c *gin.Context) {
		raw, _ := middleware.Bearer(c)
		if e := h.uc.Logout(c.Request.Context(), request.Actor(c), raw); e != nil {
			response.WriteError(c, e)
			return
		}
		h.cookie(c, "", -1)
		response.NoContent(c)
	})
	g.GET("/sessions", func(c *gin.Context) {
		rows, e := h.uc.Sessions(c.Request.Context(), request.Actor(c))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Sessions retrieved", rows)
	})
	g.DELETE("/sessions/:id", func(c *gin.Context) {
		if e := h.uc.Revoke(c.Request.Context(), request.Actor(c), c.Param("id")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	g.POST("/change-password", func(c *gin.Context) {
		var in struct {
			Old  string `json:"old_password"`
			Next string `json:"password"`
		}
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.ChangePassword(c.Request.Context(), request.Actor(c), in.Old, in.Next); e != nil {
			response.WriteError(c, e)
			return
		}
		h.cookie(c, "", -1)
		response.NoContent(c)
	})
	g.POST("/google/link", func(c *gin.Context) {
		var in struct {
			Credential string `json:"credential"`
		}
		if !request.Bind(c, &in) {
			return
		}
		if e := h.uc.LinkGoogle(c.Request.Context(), request.Actor(c), in.Credential); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
}
