package media

import (
	"context"
	"github.com/gin-gonic/gin"
	"kicau.cloud/control/internal/apperror"
	core "kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
	"kicau.cloud/control/internal/media/model"
	"strconv"
)

type Business interface {
	Pair(context.Context, string, string, string) (core.Record, error)
	Revoke(context.Context, string, string) error
	Authenticate(context.Context, string) (core.Record, error)
	Ingest(context.Context, string, model.Ingestion) (core.Record, error)
	Create(context.Context, core.Record, model.Upload) (core.Record, error)
	Status(context.Context, core.Record, string) (core.Record, error)
	Sign(context.Context, core.Record, string, model.SignPart) (model.Signed, error)
	Complete(context.Context, core.Record, string) (core.Record, error)
	Read(context.Context, string, string) (core.Record, error)
	List(context.Context, string, string, string, string, string, string, int) ([]core.Record, error)
	Download(context.Context, string, string) (string, error)
}
type Handler struct{ UC Business }

func send(c *gin.Context, v any, e error) {
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Berhasil", v)
}
func (h *Handler) Routes(users, engine *gin.RouterGroup) {
	users.POST("/media/engines/:engine_id", func(c *gin.Context) {
		var b struct {
			Arena string `json:"arena_id"`
		}
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Pair(c.Request.Context(), request.Actor(c), c.Param("engine_id"), b.Arena)
		send(c, r, e)
	})
	users.DELETE("/media/engines/:engine_id", func(c *gin.Context) {
		send(c, nil, h.UC.Revoke(c.Request.Context(), request.Actor(c), c.Param("engine_id")))
	})
	users.POST("/media/ingestions", func(c *gin.Context) {
		var b model.Ingestion
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Ingest(c.Request.Context(), request.Actor(c), b)
		send(c, r, e)
	})
	users.GET("/media/recordings", func(c *gin.Context) {
		n, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if e != nil || n < 1 || n > 100 {
			response.WriteError(c, apperror.ErrInvalid)
			return
		}
		rows, e := h.UC.List(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Query("round_id"), c.Query("session_id"), c.Query("channel_id"), c.Query("cursor"), n+1)
		if e != nil {
			send(c, nil, e)
			return
		}
		more := len(rows) > n
		next := ""
		if more {
			rows = rows[:n]
			next = core.Text(rows[len(rows)-1], "id")
		}
		send(c, gin.H{"items": rows, "has_more": more, "next_cursor": next}, nil)
	})
	users.GET("/media/recordings/:id", func(c *gin.Context) {
		r, e := h.UC.Read(c.Request.Context(), request.Actor(c), c.Param("id"))
		send(c, r, e)
	})
	users.GET("/media/recordings/:id/download", func(c *gin.Context) {
		url, e := h.UC.Download(c.Request.Context(), request.Actor(c), c.Param("id"))
		send(c, gin.H{"url": url, "expires_in": 60}, e)
	})
	engine.Use(func(c *gin.Context) {
		token, ok := middleware.Bearer(c)
		if !ok {
			c.Abort()
			response.WriteError(c, apperror.ErrUnauthorized)
			return
		}
		r, e := h.UC.Authenticate(c.Request.Context(), token)
		if e != nil {
			c.Abort()
			response.WriteError(c, e)
			return
		}
		c.Set("media_engine", r)
		c.Next()
	})
	engine.POST("/uploads", func(c *gin.Context) {
		var b model.Upload
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Create(c.Request.Context(), identity(c), b)
		send(c, r, e)
	})
	engine.GET("/uploads/:id", func(c *gin.Context) {
		r, e := h.UC.Status(c.Request.Context(), identity(c), c.Param("id"))
		send(c, r, e)
	})
	engine.POST("/uploads/:id/parts", func(c *gin.Context) {
		var b model.SignPart
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Sign(c.Request.Context(), identity(c), c.Param("id"), b)
		send(c, r, e)
	})
	engine.POST("/uploads/:id/complete", func(c *gin.Context) {
		r, e := h.UC.Complete(c.Request.Context(), identity(c), c.Param("id"))
		send(c, r, e)
	})
}
func identity(c *gin.Context) core.Record { v, _ := c.Get("media_engine"); return v.(core.Record) }
