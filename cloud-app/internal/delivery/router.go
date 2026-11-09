// Package delivery assembles routes and shared middleware.
package delivery

import (
	"net/http"
	"time"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type Handlers struct{ Health *rest.HealthHandler }
type Options struct {
	AllowedOrigins []string
	RequestTimeout time.Duration
}

func Router(opts Options, handlers Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handlers.Health.Live)
	mux.HandleFunc("GET /readyz", handlers.Health.Ready)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			w.Header().Set("Allow", "GET, HEAD")
			response.JSON(w, http.StatusMethodNotAllowed, false, "method not allowed", nil, nil)
			return
		}
		response.WriteError(w, apperror.ErrNotFound)
	})
	// Business and engine routes are intentionally unregistered until their
	// authorization usecases and persistent repositories are implemented.
	return middleware.SecurityHeaders(middleware.AccessLog(middleware.Recover(middleware.Limits(opts.RequestTimeout, middleware.CORS(opts.AllowedOrigins, mux)))))
}
