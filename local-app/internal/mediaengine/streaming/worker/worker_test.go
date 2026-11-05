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
	w := New()
	defer w.Close()
	waits := make(chan time.Duration, 1)
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
	if e := w.Launch("youtube", start, func(running bool) { states <- running }); e != nil {
		t.Fatal(e)
	}
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("restream did not start")
	}
	select {
	case d := <-waits:
		if d != 5*time.Second {
			t.Fatal(d)
		}
	case <-time.After(time.Second):
		t.Fatal("retry missing")
	}
	if e := w.Launch("youtube", start, func(bool) {}); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	advance <- struct{}{}
	select {
	case n := <-calls:
		if n != 2 {
			t.Fatal(n)
		}
	case <-time.After(time.Second):
		t.Fatal("restream not retried")
	}
	if e := w.Stop("youtube"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-p.done:
	default:
		t.Fatal("target process still running")
	}
	sawRunning := false
	for len(states) > 0 {
		if <-states {
			sawRunning = true
		}
	}
	if !sawRunning {
		t.Fatal("successful process observation missing")
	}
	if e := w.Stop("youtube"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	w.Close()
	if e := w.Launch("youtube", start, func(bool) {}); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}

func TestStopCancelsTargetDuringRetryWait(t *testing.T) {
	w := New()
	defer w.Close()
	waiting := make(chan struct{})
	w.wait = func(ctx context.Context, _ time.Duration) bool { close(waiting); <-ctx.Done(); return false }
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	if e := w.Launch("youtube", func(context.Context) (media.Process, error) { return nil, errors.New("offline") }, func(bool) {}); e != nil {
		t.Fatal(e)
	}
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("retry did not wait")
	}
	stopped := make(chan error, 1)
	go func() { stopped <- w.Stop("youtube") }()
	select {
	case e := <-stopped:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("stop blocked on timer")
	}
}
