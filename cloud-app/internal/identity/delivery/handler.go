package delivery

import (
	"context"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
	"kicau.cloud/control/internal/identity/model"
)

type AuthBusiness interface {
	Login(ctx context.Context, in model.LoginRequest) (*model.PreAuthResponse, error)
	SelectScope(ctx context.Context, in model.SelectScopeRequest) (*model.AuthTokens, error)
	SwitchScope(ctx context.Context, actorID, bearerToken string, in model.SwitchScopeRequest) (*model.AuthTokens, error)
	Refresh(ctx context.Context, refreshToken string) (*model.AuthTokens, error)
	Logout(ctx context.Context, refreshToken string) error
	Profile(ctx context.Context, actorID string) (*model.UserProfile, error)
	Sessions(ctx context.Context, actorID string) ([]model.SessionView, error)
	RevokeSession(ctx context.Context, actorID, sessionID string) error
	ChangePassword(ctx context.Context, actorID, oldPassword, newPassword string) error
}

type UserBusiness interface {
	Get(ctx context.Context, actorID, targetID string) (*model.UserView, error)
	List(ctx context.Context, actorID string, filter model.UserFilter) ([]model.UserView, error)
	Create(ctx context.Context, actorID string, req *model.CreateUserRequest) (*model.UserView, error)
	Update(ctx context.Context, actorID, targetID string, req *model.UpdateUserRequest) (*model.UserView, error)
	Delete(ctx context.Context, actorID, targetID string, expectedVersion int) error
}

type Handler struct {
	Auth       AuthBusiness
	Users      UserBusiness
	CookieName string
	Secure     bool
	Origins    []string
}

func (h *Handler) cookie(c *gin.Context, v string, age int) {
	c.SetCookie(h.CookieName, v, age, "/", "", h.Secure, true)
}

func (h *Handler) origin(c *gin.Context) bool {
	o := c.GetHeader("Origin")
	if o == "" {
		return true
	}
	for _, allowed := range h.Origins {
		if strings.EqualFold(o, allowed) {
			return true
		}
	}
	response.WriteError(c, apperror.ErrForbidden)
	return false
}

func result(c *gin.Context, data any, err error) {
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "success", data)
}

func (h *Handler) PublicRoutes(g *gin.RouterGroup) {
	g.POST("/login", func(c *gin.Context) {
		var in model.LoginRequest
		if !request.Bind(c, &in) {
			return
		}
		if in.DeviceID == "" {
			in.DeviceID = c.GetHeader("X-Device-ID")
		}
		in.IP = c.ClientIP()
		in.UserAgent = c.GetHeader("User-Agent")
		res, err := h.Auth.Login(c.Request.Context(), in)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, res, nil)
	})

	g.POST("/select-scope", func(c *gin.Context) {
		var in model.SelectScopeRequest
		if !request.Bind(c, &in) {
			return
		}
		if in.DeviceID == "" {
			in.DeviceID = c.GetHeader("X-Device-ID")
		}
		in.IP = c.ClientIP()
		in.UserAgent = c.GetHeader("User-Agent")
		t, err := h.Auth.SelectScope(c.Request.Context(), in)
		if err != nil {
			result(c, nil, err)
			return
		}
		h.cookie(c, t.RefreshToken, 30*86400)
		result(c, t, nil)
	})

	g.POST("/refresh", func(c *gin.Context) {
		if !h.origin(c) {
			return
		}
		cookieVal, _ := c.Cookie(h.CookieName)
		t, err := h.Auth.Refresh(c.Request.Context(), cookieVal)
		if err != nil {
			result(c, nil, err)
			return
		}
		h.cookie(c, t.RefreshToken, 30*86400)
		result(c, t, nil)
	})

	g.POST("/logout", func(c *gin.Context) {
		if !h.origin(c) {
			return
		}
		cookieVal, _ := c.Cookie(h.CookieName)
		err := h.Auth.Logout(c.Request.Context(), cookieVal)
		h.cookie(c, "", -1)
		result(c, nil, err)
	})
}

func (h *Handler) ProtectedRoutes(g *gin.RouterGroup) {
	g.GET("/auth/me", func(c *gin.Context) {
		v, err := h.Auth.Profile(c.Request.Context(), request.Actor(c))
		result(c, v, err)
	})

	g.POST("/auth/switch-scope", func(c *gin.Context) {
		var req model.SwitchScopeRequest
		if !request.Bind(c, &req) {
			return
		}
		bearer := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		t, err := h.Auth.SwitchScope(c.Request.Context(), request.Actor(c), strings.TrimSpace(bearer), req)
		if err != nil {
			result(c, nil, err)
			return
		}
		h.cookie(c, t.RefreshToken, 30*86400)
		result(c, t, nil)
	})

	g.POST("/auth/change-password", func(c *gin.Context) {
		var req model.ChangePasswordRequest
		if !request.Bind(c, &req) {
			return
		}
		err := h.Auth.ChangePassword(c.Request.Context(), request.Actor(c), req.OldPassword, req.NewPassword)
		result(c, nil, err)
	})

	g.GET("/auth/sessions", func(c *gin.Context) {
		v, err := h.Auth.Sessions(c.Request.Context(), request.Actor(c))
		result(c, v, err)
	})

	g.POST("/auth/sessions/:session/revoke", func(c *gin.Context) {
		err := h.Auth.RevokeSession(c.Request.Context(), request.Actor(c), c.Param("session"))
		result(c, nil, err)
	})

	g.GET("/users", func(c *gin.Context) {
		var isEO *bool
		if eoStr := c.Query("is_eo"); eoStr != "" {
			val := eoStr == "true" || eoStr == "1"
			isEO = &val
		}
		var role *string
		if r := c.Query("role"); r != "" {
			role = &r
		}
		var search *string
		if s := c.Query("search"); s != "" {
			search = &s
		}
		var cursor *string
		if cur := c.Query("cursor"); cur != "" {
			cursor = &cur
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
		filter := model.UserFilter{
			Cursor: cursor,
			Limit:  limit,
			IsEO:   isEO,
			Role:   role,
			Search: search,
		}
		v, err := h.Users.List(c.Request.Context(), request.Actor(c), filter)
		if err != nil {
			result(c, nil, err)
			return
		}
		hasMore := len(v) > limit
		if hasMore {
			v = v[:limit]
		}
		nextCursor := ""
		if len(v) > 0 {
			nextCursor = v[len(v)-1].ID
		}
		result(c, response.CursorPage[model.UserView]{
			Items:      v,
			NextCursor: nextCursor,
			HasMore:    hasMore,
		}, nil)
	})

	g.GET("/users/:id", func(c *gin.Context) {
		v, err := h.Users.Get(c.Request.Context(), request.Actor(c), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/users", func(c *gin.Context) {
		var req model.CreateUserRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Users.Create(c.Request.Context(), request.Actor(c), &req)
		result(c, v, err)
	})

	g.PATCH("/users/:id", func(c *gin.Context) {
		var req model.UpdateUserRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Users.Update(c.Request.Context(), request.Actor(c), c.Param("id"), &req)
		result(c, v, err)
	})

	g.DELETE("/users/:id", func(c *gin.Context) {
		expectedVersion, _ := strconv.Atoi(c.Query("expected_version"))
		err := h.Users.Delete(c.Request.Context(), request.Actor(c), c.Param("id"), expectedVersion)
		result(c, nil, err)
	})
}
