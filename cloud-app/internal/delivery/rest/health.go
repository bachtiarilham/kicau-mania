package rest

import (
	"context"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/delivery/rest/response"
)

type Readiness interface{ Ready(context.Context) error }
type HealthHandler struct{ usecase Readiness }

func NewHealthHandler(u Readiness) *HealthHandler { return &HealthHandler{u} }

// @Summary Proses hidup
// @Tags Health
// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
// @Produce json
// @Success 200 {object} response.Response{data=model.Record}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Failure 503 {object} response.Response
// @Failure 504 {object} response.Response
// @Router /healthz [get]

// @Summary Proses hidup
// @Tags Health
// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
// @Produce json
// @Success 200 "No response body"
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Failure 503 {object} response.Response
// @Failure 504 {object} response.Response
// @Router /healthz [head]
func (h *HealthHandler) Live(c *gin.Context) {
	response.Success(c, "Service is alive", gin.H{"status": "UP"})
}

// @Summary Kesiapan database dan skema
// @Tags Health
// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
// @Produce json
// @Success 200 {object} response.Response{data=model.Record}
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Failure 503 {object} response.Response
// @Failure 504 {object} response.Response
// @Router /readyz [get]

// @Summary Kesiapan database dan skema
// @Tags Health
// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
// @Produce json
// @Success 200 "No response body"
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Failure 404 {object} response.Response
// @Failure 409 {object} response.Response
// @Failure 500 {object} response.Response
// @Failure 503 {object} response.Response
// @Failure 504 {object} response.Response
// @Router /readyz [head]
func (h *HealthHandler) Ready(c *gin.Context) {
	if err := h.usecase.Ready(c.Request.Context()); err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Service is ready", gin.H{"status": "READY"})
}
