// Package rest exposes JSON handlers without constructing application dependencies.
package rest

import (
	"context"
	"net/http"

	"kicau.cloud/control/internal/delivery/rest/response"
)

type Readiness interface{ Ready(context.Context) error }
type HealthHandler struct{ usecase Readiness }

func NewHealthHandler(usecase Readiness) *HealthHandler { return &HealthHandler{usecase: usecase} }

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response.Success(w, "Service is alive", map[string]string{"status": "UP"})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if err := h.usecase.Ready(r.Context()); err != nil {
		response.WriteError(w, err)
		return
	}
	response.Success(w, "Service is ready", map[string]string{"status": "READY"})
}
