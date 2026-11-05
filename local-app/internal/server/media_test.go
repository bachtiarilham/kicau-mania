package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type crashingMedia struct {
	mu       sync.Mutex
	starts   int
	done     chan struct{}
	attempts chan int
}

func (s *crashingMedia) Start(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.starts++
	s.attempts <- s.starts
	if s.starts == 1 {
		return errors.New("binary missing")
	}
	s.done = make(chan struct{})
	close(s.done)
	return nil
}
func (s *crashingMedia) Done() <-chan struct{} { s.mu.Lock(); defer s.mu.Unlock(); return s.done }
func (s *crashingMedia) Close()                {}
func TestMediaFailureRestartsWithoutCancellingArena(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	media := &crashingMedia{attempts: make(chan int, 10)}
	done := superviseMedia(ctx, media, time.Millisecond)
	for i := 0; i < 3; i++ {
		select {
		case <-media.attempts:
		case <-time.After(time.Second):
			t.Fatal("media not retried")
		}
	}
	if ctx.Err() != nil {
		t.Fatal("media failure cancelled arena context")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("supervisor did not stop")
	}
}
