package delivery

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/mediaengine/saving/model"
	"kicau.local/edge/internal/mediaengine/saving/usecase"
)

type Handler struct{ uc *usecase.UseCase }

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) startSession(c *gin.Context) {
	var req model.StartSession
	if e := c.ShouldBindJSON(&req); e != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	out, e := h.uc.StartSession(c.Request.Context(), req)
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Accepted(c, "Recording session requested", views(out))
}

func (h *Handler) stopSession(c *gin.Context) {
	if e := h.uc.StopSession(c.Request.Context(), c.Param("id")); e != nil {
		response.WriteError(c, e)
		return
	}
	response.NoContent(c)
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
		out = append(out, gin.H{
			"id":           r.ID,
			"round_id":     r.RoundID,
			"channel_id":   r.ChannelID,
			"state":        r.State,
			"object_key":   r.Key,
			"attempts":     r.Attempts,
			"next_attempt": r.NextAttempt,
			"started_at":   r.StartedAt,
			"ended_at":     r.EndedAt,
		})
	}
	return out
}
