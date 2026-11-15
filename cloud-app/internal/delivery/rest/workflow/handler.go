// Package workflow is the HTTP boundary for schema-010 business flows.
package workflow

import (
	"bytes"
	"context"
	"github.com/gin-gonic/gin"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"kicau.cloud/control/internal/apperror"
	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
	input "kicau.cloud/control/internal/workflow/model"
	"net/http"
	"strconv"
)

type Business interface {
	Sessions(context.Context, string) ([]model.Record, error)
	RevokeSession(context.Context, string, string) error
	Profile(context.Context, string) (model.Record, error)
	List(context.Context, string, string, input.Filter) ([]model.Record, error)
	Get(context.Context, string, string, string, string) (model.Record, error)
	Save(context.Context, string, string, string, string, model.Record) (model.Record, error)
	Delete(context.Context, string, string, string, string, int) error
	Registration(context.Context, string, string, string, model.Record) (model.Record, error)
	History(context.Context, string, string, string, int, int) ([]model.Record, error)
	Transfer(context.Context, string, string, string, string, int) (model.Record, error)
	Ownership(context.Context, string, string) ([]model.Record, error)
	Awards(context.Context, string, string, string, int) ([]model.Record, error)
	Award(context.Context, string, string, string) (model.Record, error)
	SaveAward(context.Context, string, string, string, string, model.Record) (model.Record, error)
	DeleteAward(context.Context, string, string, string, string) error
	Certificate(context.Context, string, string, string) (string, error)
	TicketAction(context.Context, string, string, string, string, string) (model.Record, error)
	Proof(context.Context, string, string, string, []byte, string) (model.Record, error)
	ReadProof(context.Context, string, string, string) ([]byte, string, error)
	RoundAction(context.Context, string, string, string, string, string) (model.Record, error)
	RoundData(context.Context, string, string, string, string) ([]model.Record, error)
	AssignJudge(context.Context, string, string, string, string, bool) ([]model.Record, error)
	Score(context.Context, string, string, string, string, []input.Score) ([]model.Record, error)
	ReadScores(context.Context, string, string, string, string) ([]model.Record, error)
	Assignments(context.Context, string, string) ([]model.Record, error)
	AssignRole(context.Context, string, string, string, string, bool) ([]model.Record, error)
	Credentials(context.Context, string, string, string, string) error
	ChangePassword(context.Context, string, string, string) error
}
type Auth interface {
	Login(context.Context, auth.Login) (auth.Tokens, error)
	Refresh(context.Context, string) (auth.Tokens, error)
	Logout(context.Context, string) error
}
type Handler struct {
	UC         Business
	Auth       Auth
	CookieName string
	Secure     bool
	Origins    []string
}

func result(c *gin.Context, v any, e error) {
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Berhasil", v)
}
func (h *Handler) cookie(c *gin.Context, v string, age int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.CookieName, v, age, "/", "", h.Secure, true)
}
func (h *Handler) origin(c *gin.Context) bool {
	for _, o := range h.Origins {
		if c.GetHeader("Origin") == o {
			return true
		}
	}
	response.WriteError(c, apperror.ErrForbidden)
	return false
}
func (h *Handler) Public(g *gin.RouterGroup) {
	g.POST("/login", func(c *gin.Context) {
		var in auth.Login
		if !request.Bind(c, &in) {
			return
		}
		in.IP = c.ClientIP()
		in.UserAgent = c.GetHeader("User-Agent")
		t, e := h.Auth.Login(c.Request.Context(), in)
		if e != nil {
			result(c, nil, e)
			return
		}
		h.cookie(c, t.RefreshToken, 30*86400)
		result(c, t, nil)
	})
	g.POST("/refresh", func(c *gin.Context) {
		if !h.origin(c) {
			return
		}
		v, _ := c.Cookie(h.CookieName)
		t, e := h.Auth.Refresh(c.Request.Context(), v)
		if e != nil {
			result(c, nil, e)
			return
		}
		h.cookie(c, t.RefreshToken, 30*86400)
		result(c, t, nil)
	})
	// A revoked access token must not prevent clearing its refresh session.
	g.POST("/logout", func(c *gin.Context) {
		if !h.origin(c) {
			return
		}
		v, _ := c.Cookie(h.CookieName)
		e := h.Auth.Logout(c.Request.Context(), v)
		h.cookie(c, "", -1)
		result(c, nil, e)
	})
}
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/auth/sessions", func(c *gin.Context) { v, e := h.UC.Sessions(c.Request.Context(), request.Actor(c)); result(c, v, e) })
	g.POST("/auth/sessions/:session/revoke", func(c *gin.Context) {
		result(c, nil, h.UC.RevokeSession(c.Request.Context(), request.Actor(c), c.Param("session")))
	})
	g.GET("/auth/me", func(c *gin.Context) { v, e := h.UC.Profile(c.Request.Context(), request.Actor(c)); result(c, v, e) })
	g.POST("/auth/change-password", func(c *gin.Context) {
		var f struct {
			Old string `json:"old_password"`
			New string `json:"new_password"`
		}
		if !request.Bind(c, &f) {
			return
		}
		result(c, nil, h.UC.ChangePassword(c.Request.Context(), request.Actor(c), f.Old, f.New))
	})
	for _, res := range []string{"users", "birds", "bird-species", "arenas", "registrations"} {
		h.crud(g, "/"+res, res)
	}
	for _, res := range []string{"events", "categories", "rounds", "gantangan", "criteria"} {
		h.crud(g, "/arenas/:arena/"+res, res)
	}
	g.GET("/judge/rounds", h.list("judge-rounds"))
	g.GET("/arenas/:arena/judges", h.list("arena-judges"))
	for _, res := range []string{"users", "birds"} {
		g.GET("/"+res+"/:id/history", func(c *gin.Context) {
			n, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))
			after, _ := strconv.Atoi(c.DefaultQuery("after_version", "0"))
			v, e := h.UC.History(c.Request.Context(), request.Actor(c), res, c.Param("id"), after, n)
			result(c, v, e)
		})
	}
	g.POST("/birds/:id/transfer", func(c *gin.Context) {
		var f struct {
			Owner   string `json:"owner_id"`
			Notes   string `json:"notes"`
			Version int    `json:"expected_version"`
		}
		if !request.Bind(c, &f) {
			return
		}
		v, e := h.UC.Transfer(c.Request.Context(), request.Actor(c), c.Param("id"), f.Owner, f.Notes, f.Version)
		result(c, v, e)
	})
	g.GET("/birds/:id/ownership", func(c *gin.Context) {
		v, e := h.UC.Ownership(c.Request.Context(), request.Actor(c), c.Param("id"))
		result(c, v, e)
	})
	g.GET("/birds/:id/awards", func(c *gin.Context) {
		cursor, n, ok := request.Page(c)
		if !ok {
			return
		}
		v, e := h.UC.Awards(c.Request.Context(), request.Actor(c), c.Param("id"), cursor, n)
		if e != nil {
			result(c, nil, e)
			return
		}
		result(c, request.PageData(v, n), nil)
	})
	g.GET("/birds/:id/awards/:award", func(c *gin.Context) {
		v, e := h.UC.Award(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("award"))
		result(c, v, e)
	})
	for _, method := range []string{"POST", "PATCH"} {
		path := "/birds/:id/awards"
		if method == "PATCH" {
			path += "/:award"
		}
		g.Handle(method, path, func(c *gin.Context) {
			var f model.Record
			if !request.Bind(c, &f) {
				return
			}
			v, e := h.UC.SaveAward(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Param("id"), c.Param("award"), f)
			result(c, v, e)
		})
	}
	g.DELETE("/birds/:id/awards/:award", func(c *gin.Context) {
		result(c, nil, h.UC.DeleteAward(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Param("id"), c.Param("award")))
	})
	g.GET("/birds/:id/awards/:award/certificate", func(c *gin.Context) {
		v, e := h.UC.Certificate(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("award"))
		if e != nil {
			result(c, nil, e)
			return
		}
		result(c, model.Record{"url": v}, nil)
	})
	for _, action := range []string{"verify", "reject", "cancel", "disqualify", "requalify"} {
		g.POST("/registrations/:id/"+action, func(c *gin.Context) {
			var f struct {
				Reason string `json:"reason"`
			}
			if !request.Bind(c, &f) {
				return
			}
			v, e := h.UC.TicketAction(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Param("id"), action, f.Reason)
			result(c, v, e)
		})
	}
	g.POST("/registrations/:id/proof", h.upload)
	g.GET("/registrations/:id/proof", func(c *gin.Context) {
		b, m, e := h.UC.ReadProof(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Param("id"))
		if e != nil {
			result(c, nil, e)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.Header("Content-Disposition", "inline")
		c.Data(200, m, b)
	})
	base := "/arenas/:arena/rounds/:id"
	for _, action := range []string{"start", "begin-judging", "close", "cancel"} {
		g.POST(base+"/"+action, func(c *gin.Context) {
			var f struct {
				Status string `json:"expected_status"`
			}
			if !request.Bind(c, &f) {
				return
			}
			v, e := h.UC.RoundAction(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), action, f.Status)
			result(c, v, e)
		})
	}
	for _, kind := range []string{"judges", "participants", "recap"} {
		g.GET(base+"/"+kind, func(c *gin.Context) {
			v, e := h.UC.RoundData(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), kind)
			result(c, v, e)
		})
	}
	for _, method := range []string{"POST", "DELETE"} {
		g.Handle(method, base+"/judges/:judge", func(c *gin.Context) {
			v, e := h.UC.AssignJudge(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), c.Param("judge"), c.Request.Method == "DELETE")
			result(c, v, e)
		})
	}
	g.PUT(base+"/scores/:ticket", func(c *gin.Context) {
		var f struct {
			Scores []input.Score `json:"scores"`
		}
		if !request.Bind(c, &f) {
			return
		}
		v, e := h.UC.Score(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), c.Param("ticket"), f.Scores)
		result(c, v, e)
	})
	g.GET(base+"/scores/:ticket", func(c *gin.Context) {
		v, e := h.UC.ReadScores(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), c.Param("ticket"))
		result(c, v, e)
	})
	g.GET("/arenas/:arena/staff", func(c *gin.Context) {
		v, e := h.UC.Assignments(c.Request.Context(), request.Actor(c), c.Param("arena"))
		result(c, v, e)
	})
	for _, method := range []string{"POST", "DELETE"} {
		g.Handle(method, "/arenas/:arena/staff/:user/:role", func(c *gin.Context) {
			v, e := h.UC.AssignRole(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("user"), c.Param("role"), c.Request.Method == "DELETE")
			result(c, v, e)
		})
	}
	g.POST("/arenas/:arena/staff/:user/credentials", func(c *gin.Context) {
		var f struct {
			Password string `json:"password"`
		}
		if !request.Bind(c, &f) {
			return
		}
		result(c, nil, h.UC.Credentials(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("user"), f.Password))
	})
}
func (h *Handler) list(res string) gin.HandlerFunc {
	return func(c *gin.Context) {
		cursor, n, ok := request.Page(c)
		if !ok {
			return
		}
		a := c.Param("arena")
		if a == "" {
			a = c.Query("arena_id")
		}
		f := input.Filter{Arena: a, Cursor: cursor, Limit: n, Event: c.Query("event_id"), Round: c.Query("round_id"), Owner: c.Query("owner_id"), Role: c.Query("role"), EO: c.Query("is_eo"), Search: c.Query("q"), Bird: c.Query("bird_id")}
		v, e := h.UC.List(c.Request.Context(), request.Actor(c), res, f)
		if e != nil {
			result(c, nil, e)
			return
		}
		result(c, request.PageData(v, n), nil)
	}
}
func (h *Handler) crud(g *gin.RouterGroup, path, res string) {
	g.GET(path, h.list(res))
	suffix := "/:id"
	if res == "arenas" {
		suffix = "/:arena"
	}
	getID := func(c *gin.Context) string {
		if res == "arenas" {
			return c.Param("arena")
		}
		return c.Param("id")
	}
	g.GET(path+suffix, func(c *gin.Context) {
		a := c.Param("arena")
		if a == "" {
			a = c.Query("arena_id")
		}
		v, e := h.UC.Get(c.Request.Context(), request.Actor(c), res, getID(c), a)
		result(c, v, e)
	})
	for _, method := range []string{"POST", "PATCH", "PUT"} {
		p := path
		if method != "POST" {
			p += suffix
		}
		g.Handle(method, p, func(c *gin.Context) {
			var f model.Record
			if !request.Bind(c, &f) {
				return
			}
			a := c.Param("arena")
			if a == "" {
				a = c.Query("arena_id")
			}
			var v model.Record
			var e error
			if res == "registrations" {
				a = model.Text(f, "arena_id")
				delete(f, "arena_id")
				v, e = h.UC.Registration(c.Request.Context(), request.Actor(c), getID(c), a, f)
			} else {
				if v, ok := f["arena_id"]; ok {
					if v != a {
						result(c, nil, apperror.ErrInvalid)
						return
					}
					delete(f, "arena_id")
				}
				v, e = h.UC.Save(c.Request.Context(), request.Actor(c), res, getID(c), a, f)
			}
			result(c, v, e)
		})
	}
	if res != "registrations" {
		g.DELETE(path+suffix, func(c *gin.Context) {
			version, _ := strconv.Atoi(c.Query("expected_version"))
			result(c, nil, h.UC.Delete(c.Request.Context(), request.Actor(c), res, getID(c), c.Param("arena"), version))
		})
	}
}
func (h *Handler) upload(c *gin.Context) {
	if e := c.Request.ParseMultipartForm(6 << 20); e != nil {
		result(c, nil, apperror.ErrInvalid)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	f, _, e := c.Request.FormFile("file")
	if e != nil {
		result(c, nil, apperror.ErrInvalid)
		return
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, (5<<20)+1))
	if e != nil || len(b) > 5<<20 {
		result(c, nil, apperror.Public(apperror.ErrInvalid, "Gambar maksimal 5 MiB"))
		return
	}
	mime := http.DetectContentType(b)
	_, format, e := image.DecodeConfig(bytes.NewReader(b))
	if e != nil || (format != "png" && format != "jpeg") || (mime != "image/png" && mime != "image/jpeg") {
		result(c, nil, apperror.Public(apperror.ErrInvalid, "Signature gambar harus JPEG atau PNG"))
		return
	}
	v, e := h.UC.Proof(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Param("id"), b, mime)
	result(c, v, e)
}
