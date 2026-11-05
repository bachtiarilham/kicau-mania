package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/media"
)

type fakeProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { <-p.done; return nil }
func (p *fakeProcess) Stop()                 { p.once.Do(func() { close(p.done) }) }

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

func TestGuardWaitsBeforeFirstCheck(t *testing.T) {
	w := New()
	defer w.Close()
	waits := make(chan time.Duration, 1)
	advance := make(chan struct{})
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
	checks := make(chan struct{}, 1)
	if e := w.RunGuard(func(context.Context) { checks <- struct{}{} }); e != nil {
		t.Fatal(e)
	}
	select {
	case d := <-waits:
		if d != 5*time.Second {
			t.Fatal(d)
		}
	case <-time.After(time.Second):
		t.Fatal("guard wait missing")
	}
	select {
	case <-checks:
		t.Fatal("guard ran before first interval")
	default:
	}
	advance <- struct{}{}
	select {
	case <-checks:
	case <-time.After(time.Second):
		t.Fatal("guard check missing")
	}
}

func TestRecordingCompletionWaitIncludesFinalization(t *testing.T) {
	w := New()
	defer w.Close()
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	p := &fakeProcess{done: make(chan struct{})}
	entered, release := make(chan struct{}), make(chan struct{})
	// Always release a blocked callback if an assertion fails.
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	done, e := w.Watch(p, func(err error) {
		if err != nil {
			t.Error(err)
		}
		close(entered)
		<-release
	})
	if e != nil {
		t.Fatal(e)
	}
	if e = w.StopProcesses([]media.Process{p}); e != nil {
		t.Fatal(e)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("finalizer not called")
	}
	select {
	case <-done:
		t.Fatal("completion signaled before callback finished")
	default:
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = w.WaitFor(ctx, []<-chan struct{}{done}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	releaseOnce.Do(func() { close(release) })
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if e = w.WaitFor(ctx, []<-chan struct{}{done}); e != nil {
		t.Fatal(e)
	}
	w.Close()
	if _, e = w.Watch(p, func(error) { t.Error("late finalizer") }); !errors.Is(e, apperror.ErrUnavailable) {
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

func TestWorkerCloseStopsWatchedProcess(t *testing.T) {
	w := New()
	if e := w.Start(); e != nil {
		t.Fatal(e)
	}
	p := &fakeProcess{done: make(chan struct{})}
	finalized := make(chan struct{})
	done, e := w.Watch(p, func(error) { close(finalized) })
	if e != nil {
		t.Fatal(e)
	}
	closed := make(chan struct{})
	go func() { w.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("worker shutdown leaked watcher")
	}
	select {
	case <-p.done:
	default:
		t.Fatal("process survived close")
	}
	select {
	case <-done:
	default:
		t.Fatal("watcher survived close")
	}
	select {
	case <-finalized:
	default:
		t.Fatal("finalizer skipped")
	}
}
