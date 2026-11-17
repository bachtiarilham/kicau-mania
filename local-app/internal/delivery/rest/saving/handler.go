package delivery

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/saving/model"
)

// RecordingUseCase contains only the recording operations exposed by this handler.
type RecordingUseCase interface {
	Recordings(context.Context, string, int) ([]model.Recording, string, error)
}

type Handler struct{ uc RecordingUseCase }

func New(uc RecordingUseCase) *Handler {
	return &Handler{uc: uc}
}

// recordings godoc
// @Summary Daftar rekaman dengan cursor
// @Description round_id tetap nama field kompatibilitas untuk identitas sesi. Hanya metadata publik, tidak mengembalikan path file lokal atau kredensial S3.
// @Tags Recordings
// @Produce json
// @Security OperatorToken
// @Security LocalControl
// @Param cursor query string false "Cursor opaque dari next_cursor sebelumnya"
// @Param limit query int false "Ukuran halaman" default(25) minimum(1) maximum(100)
// @Success 200 {object} response.Response{data=RecordingPage}
// @Failure 400,401,403,503 {object} response.Response
// @Router /api/v1/recordings [get]
func (h *Handler) recordings(c *gin.Context) {
	limit, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if e != nil {
		response.BadRequest(c, "invalid limit syntax", nil)
		return
	}
	out, next, e := h.uc.Recordings(c.Request.Context(), c.Query("cursor"), limit)
	if filtered, ok := h.uc.(interface {
		RecordingsFiltered(context.Context, string, int, string, string, string) ([]model.Recording, string, error)
	}); ok {
		out, next, e = filtered.RecordingsFiltered(c.Request.Context(), c.Query("cursor"), limit, c.Query("session_id"), c.Query("channel_id"), c.Query("state"))
	}
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Recordings retrieved", response.CursorPage[gin.H]{Items: views(out), NextCursor: next, HasMore: next != ""})
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

// RecordingView documents the public fields produced by views, without local paths.
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

// RecordingPage documents the existing cursor envelope.
type RecordingPage struct {
	Items      []RecordingView `json:"items"`
	NextCursor string          `json:"next_cursor,omitempty"`
	HasMore    bool            `json:"has_more"`
}
