// Package response writes the REST envelope and maps application errors to HTTP.
package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/logger"
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
	switch {
	case errors.Is(err, apperror.ErrUnauthorized):
		status, message = 401, "authentication required"
	case errors.Is(err, apperror.ErrForbidden):
		status, message = 403, "operation not permitted"
	case errors.Is(err, apperror.ErrInvalid):
		status, message = 400, "invalid operation"
	case errors.Is(err, apperror.ErrConflict):
		status, message = 409, "operation conflicts with current state"
	case errors.Is(err, apperror.ErrNotFound):
		status, message = 404, "not found"
	case errors.Is(err, apperror.ErrUnavailable):
		status, message = 503, "media service or disk unavailable"
	}
	c.Set("application_error", message)
	var details any
	if status == 500 {
		logger.FromContext(c.Request.Context()).Error("Internal server error", zap.Error(err))
		details = requestDetails(c)
	} else {
		var public *apperror.Message
		if errors.As(err, &public) {
			message = public.Text
		}
	}
	JSON(c, status, false, message, nil, details)
}
