package delivery

import (
	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/competition/usecase"
	"kicau.local/edge/internal/delivery/rest/response"
)

type Rounds interface {
	Current() *usecase.Snapshot
	Start(usecase.Round) (usecase.Snapshot, error)
	Stop(string, string) (usecase.Snapshot, error)
}

type Handler struct {
	rounds Rounds
}

func New(rounds Rounds) *Handler {
	return &Handler{rounds: rounds}
}

func (h *Handler) Routes(read, control *gin.RouterGroup) {
	read.GET("/local/round", h.current)
	control.POST("/local/round/start", h.start)
	control.POST("/local/round/stop", h.stop)
}

func (h *Handler) current(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	response.Success(c, "Status ronde", h.rounds.Current())
}

func (h *Handler) start(c *gin.Context) {
	var round usecase.Round
	if c.ShouldBindJSON(&round) != nil {
		response.WriteError(c, apperror.ErrInvalid)
		return
	}
	snapshot, err := h.rounds.Start(round)
	writeSnapshot(c, snapshot, err)
}

func (h *Handler) stop(c *gin.Context) {
	var request struct {
		ArenaID string `json:"arenaId" binding:"required,max=120"`
		RoundID string `json:"roundId" binding:"required,max=120"`
	}
	if c.ShouldBindJSON(&request) != nil {
		response.WriteError(c, apperror.ErrInvalid)
		return
	}
	snapshot, err := h.rounds.Stop(request.ArenaID, request.RoundID)
	writeSnapshot(c, snapshot, err)
}

func writeSnapshot(c *gin.Context, snapshot usecase.Snapshot, err error) {
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Status ronde", snapshot)
}
