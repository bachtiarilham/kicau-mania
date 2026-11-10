// Package response writes the REST envelope and maps application errors to HTTP.
package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
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

// CursorPage preserves the application's opaque string cursor format.
type CursorPage[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

// JSON encodes before committing headers so encoding failures can return 500.
// Errors/details passed here must be safe for display, never raw internal errors.
func JSON(c *gin.Context, status int, success bool, message string, data, details any) {
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	body, err := json.Marshal(Response{Success: success, Message: message, Data: data, Errors: details})
	if err != nil {
		logger.FromContext(c.Request.Context()).Error("HTTP response encoding failed", zap.Error(err))
		status = http.StatusInternalServerError
		body, _ = json.Marshal(Response{Success: false, Message: "internal server error", Errors: requestDetails(c)})
	}
	c.Data(status, "application/json; charset=utf-8", body)
}

func Success(c *gin.Context, message string, data any)  { JSON(c, 200, true, message, data, nil) }
func Created(c *gin.Context, message string, data any)  { JSON(c, 201, true, message, data, nil) }
func Accepted(c *gin.Context, message string, data any) { JSON(c, 202, true, message, data, nil) }
func BadRequest(c *gin.Context, message string, details any) {
	JSON(c, 400, false, message, nil, details)
}
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

func requestDetails(c *gin.Context) any {
	if id := c.Writer.Header().Get("X-Request-ID"); id != "" {
		return map[string]string{"request_id": id}
	}
	return nil
}

// WriteError returns only classified/public messages. Unknown errors stay internal.
func WriteError(c *gin.Context, err error) {
	status, message := 500, "internal server error"
	for _, item := range []struct {
		err    error
		status int
	}{{apperror.ErrInvalid, 400}, {apperror.ErrUnauthorized, 401}, {apperror.ErrForbidden, 403}, {apperror.ErrNotFound, 404}, {apperror.ErrConflict, 409}, {apperror.ErrUnavailable, 503}, {apperror.ErrRateLimited, 429}} {
		if errors.Is(err, item.err) {
			status, message = item.status, item.err.Error()
			break
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		status, message = 504, "request timed out"
	} else if errors.Is(err, context.Canceled) {
		status, message = 408, "request cancelled"
	}
	var public *apperror.PublicError
	if status != 500 && errors.As(err, &public) {
		message = public.Message
	}
	var details any
	if status == 500 {
		logger.FromContext(c.Request.Context()).Error("Internal server error", zap.Error(err))
		details = requestDetails(c)
	}
	JSON(c, status, false, message, nil, details)
}
