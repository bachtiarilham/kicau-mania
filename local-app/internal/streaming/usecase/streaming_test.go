package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
	cameraRepo "kicau.local/edge/internal/camera/repository"
	"kicau.local/edge/internal/infra/media"
	"kicau.local/edge/internal/streaming/worker"
)

type fakeProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { <-p.done; return nil }
func (p *fakeProcess) Stop()                 { p.once.Do(func() { close(p.done) }) }

type restreamer struct{ started chan string }

func (f restreamer) Restream(ctx context.Context, source, _ string) (media.Process, error) {
	p := &fakeProcess{done: make(chan struct{})}
	f.started <- source
	go func() {
		select {
		case <-ctx.Done():
			p.Stop()
		case <-p.done:
		}
	}()
	return p, nil
}

func TestRestreamAllowlistAndCancellation(t *testing.T) {
	sources, e := cameraRepo.New([]cameraRepo.Input{{ID: "lapak01", Source: "rtsp://192.0.2.1/a"}, {ID: "lapak02", Source: "rtsp://192.0.2.1/b"}}, "rtsp://127.0.0.1:8554")
	if e != nil {
		t.Fatal(e)
	}
	started := make(chan string, 1)
	u := NewRestreamUseCase(sources, restreamer{started}, map[string]string{"youtube": "rtmps://example.test/live/key"}, worker.NewRestreamWorker())
	defer u.Close()
	if e = u.Start(); e != nil {
		t.Fatal(e)
	}
	if e = u.StartRestream("lapak01", "rtmps://attacker.test/key"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	if e = u.StartRestream("missing", "youtube"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	if e = u.StartRestream("lapak01", "youtube"); e != nil {
		t.Fatal(e)
	}
	select {
	case got := <-started:
		if got != "rtsp://127.0.0.1:8554/lapak01" {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("restream not started")
	}
	if e = u.StartRestream("lapak01", "youtube"); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal("duplicate start should conflict")
	}
	if e = u.StopRestream("missing"); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	if e = u.StopRestream("youtube"); e != nil {
		t.Fatal(e)
	}
	if len(u.Restreams()) != 0 {
		t.Fatal("relays not cleared")
	}
}
