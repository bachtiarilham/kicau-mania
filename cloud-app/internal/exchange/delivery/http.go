package delivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"kicau.cloud/control/internal/exchange/model"
	"kicau.cloud/control/internal/exchange/usecase"
)

type Handler struct{ uc *usecase.UseCase }

func Router(uc *usecase.UseCase) http.Handler {
	h := &Handler{uc: uc}
	r := http.NewServeMux()
	r.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "UP"}) })
	r.HandleFunc("POST /api/v1/engines/{engine}/changes", h.receive)
	r.HandleFunc("GET /api/v1/engines/{engine}/commands", h.commands)
	r.HandleFunc("POST /api/v1/engines/{engine}/commands/results", h.results)
	r.HandleFunc("POST /api/v1/operator/commands", h.enqueue)
	r.HandleFunc("GET /api/v1/operator/commands/{engine}/{command}", h.commandState)
	r.HandleFunc("POST /api/v1/operator/packages", h.prepare)
	r.HandleFunc("GET /api/v1/engines/{engine}/events/{event}/package", h.packageFile)
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		request.Body = http.MaxBytesReader(w, request.Body, 2*1024*1024)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		r.ServeHTTP(w, request)
	})
}
func (h *Handler) commandState(w http.ResponseWriter, r *http.Request) {
	v, err := h.uc.CommandState(r.Context(), bearer(r), r.PathValue("engine"), r.PathValue("command"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, v)
}
func bearer(r *http.Request) string {
	return strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
}
func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func failure(w http.ResponseWriter, err error) {
	status := 500
	switch {
	case errors.Is(err, model.ErrUnauthorized):
		status = 401
	case errors.Is(err, model.ErrInvalid):
		status = 400
	case errors.Is(err, model.ErrConflict):
		status = 409
	case errors.Is(err, model.ErrNotFound):
		status = 404
	}
	write(w, status, map[string]string{"error": http.StatusText(status)})
}

func (h *Handler) prepare(w http.ResponseWriter, r *http.Request) {
	var body json.RawMessage
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		write(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := h.uc.Prepare(r.Context(), bearer(r), body); err != nil {
		failure(w, err)
		return
	}
	write(w, 201, map[string]string{"status": "prepared_in_cloud"})
}
func (h *Handler) packageFile(w http.ResponseWriter, r *http.Request) {
	body, err := h.uc.Package(r.Context(), r.PathValue("engine"), bearer(r), r.PathValue("event"))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, body)
}
func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Changes []model.Change `json:"changes"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		write(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	ids, err := h.uc.Receive(r.Context(), r.PathValue("engine"), bearer(r), body.Changes)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"acknowledged": ids})
}
func (h *Handler) commands(w http.ResponseWriter, r *http.Request) {
	commands, err := h.uc.Commands(r.Context(), r.PathValue("engine"), bearer(r))
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"commands": commands})
}
func (h *Handler) results(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Results []model.Result `json:"results"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		write(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	ids, err := h.uc.Results(r.Context(), r.PathValue("engine"), bearer(r), body.Results)
	if err != nil {
		failure(w, err)
		return
	}
	write(w, 200, map[string]any{"acknowledged": ids})
}
func (h *Handler) enqueue(w http.ResponseWriter, r *http.Request) {
	var command model.Command
	if json.NewDecoder(r.Body).Decode(&command) != nil {
		write(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if err := h.uc.Enqueue(r.Context(), bearer(r), command); err != nil {
		failure(w, err)
		return
	}
	write(w, 202, map[string]string{"status": "PENDING"})
}
