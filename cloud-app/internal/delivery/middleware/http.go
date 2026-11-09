package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"time"

	"go.uber.org/zap"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/delivery/rest/response"
	"kicau.cloud/control/internal/infra/logger"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			var bytes [16]byte
			if _, err := rand.Read(bytes[:]); err != nil {
				response.WriteError(w, err)
				return
			}
			id = hex.EncodeToString(bytes[:])
		}
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(logger.WithRequestID(r.Context(), id))
		recorded := &statusWriter{ResponseWriter: w}
		start := time.Now()
		next.ServeHTTP(recorded, r)
		status := recorded.status
		if status == 0 {
			status = http.StatusOK
		}
		// Use the registered pattern only; raw paths may contain session secrets.
		logger.FromContext(r.Context()).Info("HTTP request", zap.String("method", r.Method), zap.String("route", r.Pattern), zap.Int("status", status), zap.Duration("latency", time.Since(start)))
	})
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				logger.FromContext(r.Context()).Error("HTTP panic recovered")
				if recorded, ok := w.(*statusWriter); ok && recorded.status != 0 {
					return
				}
				response.WriteError(w, nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func Limits(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 2*1024*1024)
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		request := r.WithContext(ctx)
		next.ServeHTTP(w, request)
		// ServeMux writes the matched pattern on the request it receives.
		// Propagate it to the outer access logger without logging raw paths.
		r.Pattern = request.Pattern
	})
}

func CORS(origins []string, next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range origins {
		allowed[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		if origin := r.Header.Get("Origin"); origin != "" {
			if !allowed[origin] {
				response.WriteError(w, apperror.ErrForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
