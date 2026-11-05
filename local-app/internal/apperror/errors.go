package apperror

import "errors"

var (
	ErrInvalid     = errors.New("invalid operation")
	ErrConflict    = errors.New("operation conflicts with current state")
	ErrNotFound    = errors.New("not found")
	ErrUnavailable = errors.New("media service unavailable")
)
