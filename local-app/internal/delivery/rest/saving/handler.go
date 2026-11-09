package delivery

import (
	"context"
	"strconv"

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

func (h *Handler) recordings(c *gin.Context) {
	limit, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if e != nil {
		response.BadRequest(c, "invalid limit syntax", nil)
		return
	}
	out, next, e := h.uc.Recordings(c.Request.Context(), c.Query("cursor"), limit)
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
