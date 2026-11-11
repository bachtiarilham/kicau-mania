// Package apperror defines shared application errors independent of transport.
package apperror

import "errors"

// Message is safe for display at the HTTP boundary; causes remain inspectable.
type Message struct {
	Code string
	Kind error
	Text string
}

func (e *Message) Error() string           { return e.Text }
func (e *Message) Unwrap() error           { return e.Kind }
func Public(kind error, text string) error { return &Message{Kind: kind, Text: text} }
func Diagnostic(kind error, code, text string) error {
	return &Message{Kind: kind, Code: code, Text: text}
}

var (
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("operation not permitted")
	ErrInvalid      = errors.New("invalid operation")
	ErrConflict     = errors.New("operation conflicts with current state")
	ErrNotFound     = errors.New("not found")
	ErrUnavailable  = errors.New("media service unavailable")
)
