// Package rest handles JSON REST endpoints, independently of protocol routing.
package rest

import (
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
)

// Health godoc
// @Summary Periksa proses API hidup
// @Tags Health
// @Produce json
// @Success 200 {object} response.Response{data=HealthResponse}
// @Router /healthz [get]
func Health(c *gin.Context) {
	response.Success(c, "Service is alive", gin.H{"status": "UP"})
}

// HealthResponse describes process liveness, independently of media readiness.
type HealthResponse struct {
	Status string `json:"status" example:"UP"`
}
