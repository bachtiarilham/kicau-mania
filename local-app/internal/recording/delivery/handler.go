package delivery

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/recording/model"
)

type SessionUseCase interface {
	StartSession(context.Context, model.StartRequest) (model.Session, error)
	StopSession(context.Context, string) (model.Session, error)
	Get(context.Context, string) (model.Session, error)
	Current(context.Context) (*model.Session, error)
	Reset(context.Context) error
}

type SavingUseCase interface {
	Recordings(context.Context, string, int) ([]model.Recording, string, error)
	RecordingsFiltered(context.Context, string, int, string, string, string) ([]model.Recording, string, error)
}

type Handler struct {
	sessions SessionUseCase
	saving   SavingUseCase
}

func NewHandler(sessions SessionUseCase, saving SavingUseCase) *Handler {
	return &Handler{
		sessions: sessions,
		saving:   saving,
	}
}

func (h *Handler) Routes(g *gin.RouterGroup) {
	if g == nil {
		return
	}
	if h.sessions != nil {
		g.POST("/media/sessions", h.StartSession)
		g.POST("/media/sessions/reset", h.ResetSession)
		g.POST("/media/sessions/:id/stop", h.StopSession)
		g.GET("/media/sessions/current", h.CurrentSession)
		g.GET("/media/sessions/:id", h.GetSession)
	}
	if h.saving != nil {
		g.GET("/recordings", h.Recordings)
	}
}

func (h *Handler) StartSession(c *gin.Context) {
	var req model.StartRequest
	if c.ShouldBindJSON(&req) != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	s, err := h.sessions.StartSession(c.Request.Context(), req)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Accepted(c, "Media session accepted", s)
}

func (h *Handler) StopSession(c *gin.Context) {
	s, err := h.sessions.StopSession(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Accepted(c, "Media session stop accepted", s)
}

func (h *Handler) GetSession(c *gin.Context) {
	s, err := h.sessions.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Media session retrieved", s)
}

func (h *Handler) CurrentSession(c *gin.Context) {
	s, err := h.sessions.Current(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	if s == nil {
		response.Success(c, "No session active", nil)
		return
	}
	response.Success(c, "Current media session", s)
}

func (h *Handler) ResetSession(c *gin.Context) {
	if err := h.sessions.Reset(c.Request.Context()); err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Media sessions reset", nil)
}

func (h *Handler) Recordings(c *gin.Context) {
	limit, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if e != nil {
		response.BadRequest(c, "invalid limit syntax", nil)
		return
	}
	out, next, e := h.saving.RecordingsFiltered(c.Request.Context(), c.Query("cursor"), limit, c.Query("session_id"), c.Query("channel_id"), c.Query("state"))
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Recordings retrieved", response.CursorPage[gin.H]{
		Items:      views(out),
		NextCursor: next,
		HasMore:    next != "",
	})
}

func views(records []model.Recording) []gin.H {
	out := []gin.H{}
	for _, r := range records {
		view := gin.H{
			"id":           r.ID,
			"round_id":     r.RoundID,
			"channel_id":   r.ChannelID,
			"state":        r.State,
			"object_key":   r.Key,
			"attempts":     r.Attempts,
			"next_attempt": r.NextAttempt,
			"started_at":   r.StartedAt,
			"ended_at":     r.EndedAt,
		}
		if len(r.Segments) > 0 {
			view["segment_count"] = len(r.Segments)
		}
		if len(r.Gaps) > 0 {
			view["gaps"] = r.Gaps
		}
		if r.Error != "" {
			view["error"] = r.Error
		}
		out = append(out, view)
	}
	return out
}

type RecordingView struct {
	ID           string      `json:"id"`
	RoundID      string      `json:"round_id"`
	ChannelID    string      `json:"channel_id"`
	State        string      `json:"state"`
	ObjectKey    string      `json:"object_key"`
	Attempts     int         `json:"attempts"`
	NextAttempt  time.Time   `json:"next_attempt"`
	StartedAt    time.Time   `json:"started_at"`
	EndedAt      time.Time   `json:"ended_at"`
	SegmentCount int         `json:"segment_count,omitempty"`
	Gaps         []model.Gap `json:"gaps,omitempty"`
	Error        string      `json:"error,omitempty"`
}

type RecordingPage struct {
	Items      []RecordingView `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}
