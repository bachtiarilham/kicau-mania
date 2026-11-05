package delivery

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/http/response"
	"kicau.local/edge/internal/saving/model"
	"kicau.local/edge/internal/saving/usecase"
)

type Handler struct{ uc *usecase.UseCase }

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) startRound(c *gin.Context) {
	var req model.StartRound
	if e := c.ShouldBindJSON(&req); e != nil {
		c.JSON(400, gin.H{"error": "invalid request syntax"})
		return
	}
	out, e := h.uc.StartRound(c.Request.Context(), req)
	if e != nil {
		response.WriteError(c, e)
		return
	}
	c.JSON(202, gin.H{"data": views(out)})
}

func (h *Handler) stopRound(c *gin.Context) {
	if e := h.uc.StopRound(c.Request.Context(), c.Param("id")); e != nil {
		response.WriteError(c, e)
		return
	}
	c.Status(204)
}

func (h *Handler) recordings(c *gin.Context) {
	limit, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
	if e != nil {
		c.JSON(400, gin.H{"error": "invalid limit syntax"})
		return
	}
	out, next, e := h.uc.Recordings(c.Request.Context(), c.Query("cursor"), limit)
	if e != nil {
		response.WriteError(c, e)
		return
	}
	c.JSON(200, gin.H{"data": views(out), "next_cursor": next})
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
