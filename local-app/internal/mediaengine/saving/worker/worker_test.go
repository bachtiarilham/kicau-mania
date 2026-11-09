package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
)

func TestUploadScheduleIsImmediateThenSequential(t *testing.T) {
	w := New()
	defer w.Close()
	waits := make(chan time.Duration, 1)
	advance := make(chan struct{})
	release := make(chan struct{}, 2)
	w.wait = func(ctx context.Context, d time.Duration) bool {
		select {
		case waits <- d:
		case <-ctx.Done():
			return false
		}
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
	calls := make(chan struct{}, 2)
	if e := w.StartUploadLoop(func(ctx context.Context) {
		calls <- struct{}{}
		select {
		case <-ctx.Done():
		case <-release:
		}
	}); e != nil {
		t.Fatal(e)
	}
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("upload did not run immediately")
	}
	select {
	case <-waits:
		t.Fatal("scheduled next upload while previous callback active")
	default:
	}
	release <- struct{}{}
	select {
	case d := <-waits:
		if d != 5*time.Second {
			t.Fatal(d)
		}
	case <-time.After(time.Second):
		t.Fatal("upload retry schedule missing")
	}
	advance <- struct{}{}
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("second upload callback missing")
	}
	w.Close()
	if e := w.StartUploadLoop(func(context.Context) { t.Error("late callback") }); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}

func TestUploadTimeoutAndCancellation(t *testing.T) {
	w := New()
	defer w.Close()
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	if e := w.UploadWithTimeout(context.Background(), func(ctx context.Context) error {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Minute || time.Until(deadline) < 29*time.Minute {
			t.Error("upload timeout changed")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e := w.UploadWithTimeout(ctx, func(context.Context) error { t.Error("canceled upload invoked callback"); return nil }); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	w.Close()
	if e := w.UploadWithTimeout(context.Background(), func(context.Context) error { t.Error("upload after close"); return nil }); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}
