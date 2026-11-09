// Package response writes the REST envelope and maps application errors to HTTP.
package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/infra/logger"
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	Errors  any    `json:"errors,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Tag     string `json:"tag"`
	Message string `json:"message"`
}

type CursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// JSON marshals before writing headers. Details must be safe for client display.
func JSON(w http.ResponseWriter, status int, success bool, message string, data, details any) {
	if status == http.StatusNoContent {
		w.WriteHeader(status)
		return
	}
	body, err := json.Marshal(Response{Success: success, Message: message, Data: data, Errors: details})
	if err != nil {
		logger.Error("HTTP response encoding failed", zap.Error(err), zap.String("requestId", w.Header().Get("X-Request-ID")))
		status = http.StatusInternalServerError
		body, _ = json.Marshal(Response{Success: false, Message: "internal server error", Errors: requestDetails(w)})
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func Success(w http.ResponseWriter, message string, data any) { JSON(w, 200, true, message, data, nil) }
func Created(w http.ResponseWriter, message string, data any) { JSON(w, 201, true, message, data, nil) }
func Accepted(w http.ResponseWriter, message string, data any) {
	JSON(w, 202, true, message, data, nil)
}
func BadRequest(w http.ResponseWriter, message string, details any) {
	JSON(w, 400, false, message, nil, details)
}
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

func requestDetails(w http.ResponseWriter) any {
	if id := w.Header().Get("X-Request-ID"); id != "" {
		return map[string]string{"request_id": id}
	}
	return nil
}

func WriteError(w http.ResponseWriter, err error) {
	status, message := http.StatusInternalServerError, "internal server error"
	for _, item := range []struct {
		err    error
		status int
	}{
		{apperror.ErrInvalid, 400}, {apperror.ErrUnauthorized, 401},
		{apperror.ErrForbidden, 403}, {apperror.ErrNotFound, 404},
		{apperror.ErrConflict, 409}, {apperror.ErrUnavailable, 503},
	} {
		if errors.Is(err, item.err) {
			status, message = item.status, item.err.Error()
			break
		}
	}
	var details any
	if status == http.StatusInternalServerError {
		logger.Error("Internal server error", zap.Error(err), zap.String("requestId", w.Header().Get("X-Request-ID")))
		details = requestDetails(w)
	}
	JSON(w, status, false, message, nil, details)
}
