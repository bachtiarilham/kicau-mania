package s3

import (
	"context"
	"sync"
)

// LazyUploader keeps SDK/provider initialization out of arena startup. Failed
// initialization is retried by the recording upload loop after reconnect.
type LazyUploader struct {
	opts   Options
	mu     sync.Mutex
	client *Uploader
	create func(context.Context, Options) (*Uploader, error)
}

func NewLazy(opts Options) *LazyUploader { return &LazyUploader{opts: opts, create: New} }
func (l *LazyUploader) Put(ctx context.Context, key, file string) error {
	l.mu.Lock()
	if l.client == nil {
		client, err := l.create(ctx, l.opts)
		if err != nil {
			l.mu.Unlock()
			return err
		}
		l.client = client
	}
	client := l.client
	l.mu.Unlock()
	return client.Put(ctx, key, file)
}
