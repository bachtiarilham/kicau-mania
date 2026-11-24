// Package media defines contracts shared by media services and workers.
package media

// Process is a running media operation, independent of its OS implementation.
type Process interface {
	Done() <-chan struct{}
	Err() error
	Stop()
}

// FinalizationError contains only safe diagnostic text, never paths or process output.
type FinalizationError string

func (e FinalizationError) Error() string { return string(e) }

// PipelineError carries a stable, credential-free diagnostic to the session.
type PipelineError struct{ Code, Message string }

func (e *PipelineError) Error() string { return e.Message }
