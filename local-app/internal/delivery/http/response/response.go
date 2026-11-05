package response

import (
	"errors"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/apperror"
)

func WriteError(c *gin.Context, e error) {
	status, message := 500, "internal server error"
	switch {
	case errors.Is(e, apperror.ErrUnauthorized):
		status, message = 401, "authentication required"
	case errors.Is(e, apperror.ErrForbidden):
		status, message = 403, "operation not permitted"
	case errors.Is(e, apperror.ErrInvalid):
		status, message = 400, "invalid operation"
	case errors.Is(e, apperror.ErrConflict):
		status, message = 409, "operation conflicts with current state"
	case errors.Is(e, apperror.ErrNotFound):
		status, message = 404, "not found"
	case errors.Is(e, apperror.ErrUnavailable):
		status, message = 503, "media service or disk unavailable"
	}
	var public *apperror.Message
	if errors.As(e, &public) && status != 500 {
		message = public.Text
	}
	c.JSON(status, gin.H{"error": message})
}
