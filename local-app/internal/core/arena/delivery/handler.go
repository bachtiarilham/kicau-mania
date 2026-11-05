// Package delivery serves the offline arena API and self-contained local UI.
package delivery

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/arena/usecase"
	"kicau.local/edge/internal/delivery/http/response"
)

//go:embed web/*
var web embed.FS

type Handler struct {
	uc     *usecase.UseCase
	secure bool
}

func New(uc *usecase.UseCase, secure bool) *Handler { return &Handler{uc: uc, secure: secure} }
func token(c *gin.Context) string                   { cookie, _ := c.Cookie("arena_session"); return cookie }
func (h *Handler) catalog(c *gin.Context) {
	v, err := h.uc.Catalog(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) login(c *gin.Context) {
	var req struct {
		EventID  string `json:"event_id"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	session, expires, err := h.uc.Login(c.Request.Context(), req.EventID, req.Username, req.Password, c.ClientIP())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "arena_session", Value: session, Path: "/api/v1/arena", Expires: expires, MaxAge: int(time.Until(expires).Seconds()), Secure: h.secure, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	c.JSON(200, gin.H{"expires_at": expires})
}
func (h *Handler) logout(c *gin.Context) {
	if err := h.uc.Logout(c.Request.Context(), token(c)); err != nil {
		response.WriteError(c, err)
		return
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: "arena_session", Path: "/api/v1/arena", Value: "", MaxAge: -1, Secure: h.secure, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	c.Status(204)
}
func (h *Handler) me(c *gin.Context) {
	v, err := h.uc.JudgeView(c.Request.Context(), token(c))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) states(c *gin.Context) {
	v, err := h.uc.JudgeStates(c.Request.Context(), token(c))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) submit(c *gin.Context) {
	var req model.Submission
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	v, err := h.uc.Submit(c.Request.Context(), token(c), c.Param("round"), req)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v, "stored_in": "arena"})
}
func (h *Handler) prepare(c *gin.Context) {
	var req model.Bundle
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	if err := h.uc.Prepare(c.Request.Context(), req); err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(201, gin.H{"status": "prepared"})
}
func (h *Handler) pull(c *gin.Context) {
	var req struct {
		EventID string `json:"event_id"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	if err := h.uc.Pull(c.Request.Context(), req.EventID); err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(201, gin.H{"status": "prepared"})
}
func (h *Handler) event(c *gin.Context) {
	v, err := h.uc.OperatorView(c.Request.Context(), c.Param("event"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) history(c *gin.Context) {
	v, err := h.uc.History(c.Request.Context(), c.Param("event"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (h *Handler) transition(c *gin.Context) {
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	if err := h.uc.Transition(c.Request.Context(), c.Param("event"), c.Param("round"), req.Action, req.Reason); err != nil {
		response.WriteError(c, err)
		return
	}
	c.Status(204)
}
func (h *Handler) pending(c *gin.Context) {
	n, err := h.uc.Pending(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, gin.H{"pending_changes": n})
}
func (h *Handler) status(c *gin.Context) {
	v, err := h.uc.Status(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(200, v)
}
func (h *Handler) backup(c *gin.Context) {
	_, err := h.uc.Backup(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	c.JSON(201, gin.H{"status": "snapshot_created"})
}

func RegisterRoutes(r *gin.Engine, h *Handler, operator, origin gin.HandlerFunc) {
	assets, _ := fs.Sub(web, "web")
	r.GET("/arena", func(c *gin.Context) { c.Redirect(307, "/arena/") })
	r.GET("/arena/*path", gin.WrapH(http.StripPrefix("/arena/", http.FileServer(http.FS(assets)))))
	g := r.Group("/api/v1/arena", origin, func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
	g.GET("/events", h.catalog)
	g.GET("/status", h.status)
	g.POST("/login", h.login)
	g.POST("/logout", h.logout)
	g.GET("/me", h.me)
	g.GET("/me/status", h.states)
	g.POST("/rounds/:round/scores", h.submit)
	admin := g.Group("/operator", operator)
	admin.POST("/events", h.prepare)
	admin.POST("/packages/pull", h.pull)
	admin.GET("/events/:event", h.event)
	admin.GET("/events/:event/revisions", h.history)
	admin.POST("/events/:event/rounds/:round/transition", h.transition)
	admin.GET("/sync", h.pending)
	admin.POST("/backup", h.backup)
}
