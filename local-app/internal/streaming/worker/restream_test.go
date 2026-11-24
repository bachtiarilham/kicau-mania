package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/mediaengine/media"
)

type fakeProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { <-p.done; return nil }
func (p *fakeProcess) Stop()                 { p.once.Do(func() { close(p.done) }) }

func TestRestreamRetryAndTargetStop(t *testing.T) {
	w := NewRestreamWorker()
	defer w.Close()
	waits := make(chan time.Duration, 4)
	advance := make(chan struct{})
	w.wait = func(ctx context.Context, d time.Duration) bool {
		waits <- d
		select {
		case <-ctx.Done():
			return false
		case <-advance:
			return true
		}
	}
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	calls := make(chan int, 2)
	states := make(chan bool, 8)
	attempt := 0
	p := &fakeProcess{done: make(chan struct{})}
	start := func(context.Context) (media.Process, error) {
		attempt++
		calls <- attempt
		if attempt == 1 {
			return nil, errors.New("offline")
		}
		return p, nil
	}
	if e := w.Launch("youtube", start, func(ok bool) { states <- ok }); e != nil {
		t.Fatal(e)
	}
	if got := <-calls; got != 1 {
		t.Fatal(got)
	}
	if got := <-states; got {
		t.Fatal("expected failure notification")
	}
	if got := <-states; got {
		t.Fatal("expected process stop notification")
	}
	select {
	case d := <-waits:
		if d != 5*time.Second {
			t.Fatal(d)
		}
	case <-time.After(time.Second):
		t.Fatal("retry delay missing")
	}
	advance <- struct{}{}
	if got := <-calls; got != 2 {
		t.Fatal(got)
	}
	if got := <-states; !got {
		t.Fatal("expected success notification")
	}
	p.Stop()
	if got := <-states; got {
		t.Fatal("expected down notification")
	}
	advance <- struct{}{}
	if got := <-calls; got != 3 {
		t.Fatal(got)
	}
	if e := w.Stop("youtube"); e != nil {
		t.Fatal(e)
	}
	if e := w.Stop("missing"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	if e := w.Launch("youtube", start, func(bool) {}); e != nil {
		t.Fatal(e)
	}
	if e := w.Launch("youtube", start, func(bool) {}); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	w.Close()
	if e := w.Launch("youtube", start, func(bool) {}); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}
