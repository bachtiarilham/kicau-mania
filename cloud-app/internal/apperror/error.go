// Package apperror defines errors shared by application and delivery layers.
package apperror

import "errors"

var (
	ErrInvalid      = errors.New("invalid request")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("access denied")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnavailable  = errors.New("service unavailable")
)
