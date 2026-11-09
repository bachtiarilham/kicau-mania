// Package delivery exposes engine readiness and operational status.
package delivery

import (
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/cloud/usecase"
	"kicau.local/edge/internal/delivery/rest/response"
	"net/http"
)

// EngineUseCase exposes operational status without requiring a concrete engine.
type EngineUseCase interface {
	Status() usecase.EngineStatus
}

type Handler struct{ engine EngineUseCase }

func New(engine EngineUseCase) *Handler { return &Handler{engine: engine} }
func (h *Handler) Ready(c *gin.Context) {
	status := h.engine.Status()
	code := http.StatusOK
	if !status.Ready {
		code = http.StatusServiceUnavailable
	}
	message := "Engine is ready"
	if !status.Ready {
		message = "Engine is not ready"
	}
	response.JSON(c, code, status.Ready, message, status, nil)
}
func (h *Handler) Status(c *gin.Context) {
	response.Success(c, "Engine status retrieved", h.engine.Status())
}
