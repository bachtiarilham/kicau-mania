package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
)

func TestCaptureReconnectAndCancel(t *testing.T) {
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
	attempt := 0
	capture := func(ctx context.Context) error {
		attempt++
		calls <- attempt
		if attempt == 1 {
			return errors.New("offline")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	if e := w.StartCaptureLoop("one", capture); e != nil {
		t.Fatal(e)
	}
	select {
	case <-calls:
	case <-time.After(time.Second):
		t.Fatal("capture not started")
	}
	select {
	case d := <-waits:
		if d != 5*time.Second {
			t.Fatal(d)
		}
	case <-time.After(time.Second):
		t.Fatal("reconnect not scheduled")
	}
	advance <- struct{}{}
	select {
	case n := <-calls:
		if n != 2 {
			t.Fatal(n)
		}
	case <-time.After(time.Second):
		t.Fatal("capture not retried")
	}
	w.Close()
	if e := w.StartCaptureLoop("one", capture); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}

func TestMetricQueueIsBoundedAndBestEffort(t *testing.T) {
	w := New()
	defer w.Close()
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 128; i++ {
		if !w.EnqueueMetric([]byte("metric")) {
			t.Fatal("queue filled too early", i)
		}
	}
	if w.EnqueueMetric([]byte("overflow")) {
		t.Fatal("queue exceeded 128 items")
	}
	w.Close()
	if w.EnqueueMetric([]byte("late")) {
		t.Fatal("queue accepted item after shutdown")
	}
}

func TestBackendFailuresConsumeOnceAndShutdownCancelsSend(t *testing.T) {
	w := New()
	defer w.Close()
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	sent := make(chan string, 3)
	sender := func(ctx context.Context, b []byte) error {
		sent <- string(b)
		if string(b) == "blocked" {
			<-ctx.Done()
			return ctx.Err()
		}
		return errors.New("backend offline")
	}
	if e := w.StartMetricDeliveryLoop(sender); e != nil {
		t.Fatal(e)
	}
	if e := w.StartMetricDeliveryLoop(sender); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	w.EnqueueMetric([]byte("one"))
	w.EnqueueMetric([]byte("two"))
	w.EnqueueMetric([]byte("blocked"))
	for _, want := range []string{"one", "two", "blocked"} {
		select {
		case got := <-sent:
			if got != want {
				t.Fatalf("got %s want %s", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("backend stalled")
		}
	}
	done := make(chan struct{})
	go func() { w.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("backend shutdown stalled")
	}
	if e := w.StartMetricDeliveryLoop(sender); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
}
