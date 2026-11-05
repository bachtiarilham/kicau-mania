// Package media defines contracts shared by media services and workers.
package media

// Process is a running media operation, independent of its OS implementation.
type Process interface {
	Done() <-chan struct{}
	Err() error
	Stop()
}
