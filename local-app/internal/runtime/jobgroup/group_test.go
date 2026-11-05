package jobgroup

import (
	"context"
	"errors"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
)

func TestAdmissionAndClose(t *testing.T) {
	g := New()
	defer g.Close()
	if e := g.Go(func(context.Context) { t.Error("job ran before Start") }); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
	if e := g.Start(); e != nil {
		t.Fatal(e)
	}
	if e := g.Start(); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	entered, finished := make(chan struct{}), make(chan struct{})
	if e := g.Go(func(ctx context.Context) { close(entered); <-ctx.Done(); close(finished) }); e != nil {
		t.Fatal(e)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("job not started")
	}
	g.Close()
	g.Close()
	select {
	case <-finished:
	default:
		t.Fatal("Close returned before job finished")
	}
	if e := g.Start(); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
	if e := g.Go(func(context.Context) { t.Error("job admitted after Close") }); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}
