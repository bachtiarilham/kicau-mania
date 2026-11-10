// Package apperror classifies public failures without exposing persistence details.
package apperror

import "errors"

var (
	ErrInvalid      = errors.New("invalid request")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("access denied")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnavailable  = errors.New("service unavailable")
	ErrRateLimited  = errors.New("too many requests")
)

type PublicError struct {
	Kind    error
	Message string
}

func (e *PublicError) Error() string          { return e.Message }
func (e *PublicError) Unwrap() error          { return e.Kind }
func Public(kind error, message string) error { return &PublicError{kind, message} }
func Invalid(message string) error            { return Public(ErrInvalid, message) }
