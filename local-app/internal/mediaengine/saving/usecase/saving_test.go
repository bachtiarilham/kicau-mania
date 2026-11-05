package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/storage/filesystem"
	channelRepo "kicau.local/edge/internal/mediaengine/channel/repository"
	"kicau.local/edge/internal/mediaengine/media"
	"kicau.local/edge/internal/mediaengine/saving/model"
	"kicau.local/edge/internal/mediaengine/saving/repository"
	"kicau.local/edge/internal/mediaengine/saving/worker"
)

type fakeProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { <-p.done; return nil }
func (p *fakeProcess) Stop()                 { p.once.Do(func() { close(p.done) }) }

type fakeMedia struct{ fail bool }
type testDisk struct {
	filesystem.Disk
	space atomic.Uint64
}

func (d *testDisk) Free(string) (uint64, error) { return d.space.Load(), nil }

func (f fakeMedia) Record(_ context.Context, _, file string) (media.Process, error) {
	if f.fail {
		return nil, errors.New("unavailable")
	}
	if e := os.WriteFile(file, []byte("fixture"), 0600); e != nil {
		return nil, e
	}
	return &fakeProcess{done: make(chan struct{})}, nil
}
func fixture(t *testing.T, m fakeMedia) (*UseCase, *repository.Files) {
	t.Helper()
	dir := t.TempDir()
	r, e := repository.NewFiles(filepath.Join(dir, "queue"))
	if e != nil {
		t.Fatal(e)
	}
	sources, e := channelRepo.New([]channelRepo.Input{{ID: "lapak01", Source: "rtsp://192.0.2.1/a"}, {ID: "lapak02", Source: "rtsp://192.0.2.1/b"}}, "rtsp://127.0.0.1:8554")
	if e != nil {
		t.Fatal(e)
	}
	disk := &testDisk{}
	disk.space.Store(1000)
	return New(Options{DataDir: dir, MaxRound: time.Hour, MinFreeBytes: 100}, sources, r, m, nil, disk, worker.New()), r
}
func TestRoundLifecycleAndBusinessValidation(t *testing.T) {
	u, r := fixture(t, fakeMedia{})
	if e := u.Start(); e != nil {
		t.Fatal(e)
	}
	defer u.Close()
	ctx := context.Background()
	for _, req := range []model.StartRound{{ID: "../escape", Channels: []string{"lapak01"}}, {ID: "round1", Channels: []string{"lapak01", "lapak01"}}} {
		if _, e := u.StartRound(ctx, req); !errors.Is(e, apperror.ErrInvalid) {
			t.Fatalf("expected invalid, got %v", e)
		}
	}
	if _, e := u.StartRound(ctx, model.StartRound{ID: "round1", Channels: []string{"missing"}}); !errors.Is(e, apperror.ErrNotFound) {
		t.Fatal(e)
	}
	req := model.StartRound{ID: "round1", Channels: []string{"lapak01", "lapak02"}}
	out, e := u.StartRound(ctx, req)
	if e != nil || len(out) != 2 {
		t.Fatalf("start: %v %v", out, e)
	}
	if _, e = u.StartRound(ctx, model.StartRound{ID: "round2", Channels: []string{"lapak01"}}); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	if e = u.StopRound(ctx, "round1"); e != nil {
		t.Fatal(e)
	}
	saved, e := r.List(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range saved {
		if v.State != "READY" {
			t.Fatalf("unexpected %+v", v)
		}
		if _, e = os.Stat(v.File); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = u.StartRound(ctx, req); !errors.Is(e, apperror.ErrConflict) {
		t.Fatal(e)
	}
	page, next, e := u.Recordings(ctx, "", 1)
	if e != nil || len(page) != 1 || next == "" {
		t.Fatalf("pagination: %v %s %v", page, next, e)
	}
	page, next, e = u.Recordings(ctx, next, 1)
	if e != nil || len(page) != 1 || next != "" {
		t.Fatalf("pagination end: %v %s %v", page, next, e)
	}
}
func TestCrashRecoveryNeverUploadsPartial(t *testing.T) {
	u, r := fixture(t, fakeMedia{})
	ctx := context.Background()
	final := filepath.Join(u.cfg.DataDir, "finished.mp4")
	if e := os.WriteFile(final, []byte("finalized fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, rec := range []model.Recording{{ID: "interrupted", State: "RECORDING", File: filepath.Join(u.cfg.DataDir, "lost.mp4")}, {ID: "finalized", State: "RECORDING", File: final}} {
		if e := r.Save(ctx, rec); e != nil {
			t.Fatal(e)
		}
	}
	if e := u.Start(); e != nil {
		t.Fatal(e)
	}
	defer u.Close()
	records, e := r.List(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range records {
		expected := "INTERRUPTED"
		if v.ID == "finalized" {
			expected = "READY"
		}
		if v.State != expected {
			t.Fatalf("%s state %s", v.ID, v.State)
		}
	}
}
func TestDiskGuardAndSpawnFailure(t *testing.T) {
	u, r := fixture(t, fakeMedia{fail: true})
	if e := u.Start(); e != nil {
		t.Fatal(e)
	}
	defer u.Close()
	ctx := context.Background()
	u.disk.(*testDisk).space.Store(0)
	if _, e := u.StartRound(ctx, model.StartRound{ID: "a", Channels: []string{"lapak01"}}); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
	rows, _ := r.List(ctx)
	if len(rows) != 0 {
		t.Fatal("disk guard created records")
	}
	u.disk.(*testDisk).space.Store(1000)
	if _, e := u.StartRound(ctx, model.StartRound{ID: "a", Channels: []string{"lapak01", "lapak02"}}); !errors.Is(e, apperror.ErrUnavailable) {
		t.Fatal(e)
	}
	rows, _ = r.List(ctx)
	if len(rows) != 2 {
		t.Fatal(rows)
	}
	for _, v := range rows {
		if v.State != "FAILED" {
			t.Fatal(v)
		}
	}
}

type uploadFunc func(context.Context, string, string) error

func (f uploadFunc) Put(ctx context.Context, key, file string) error { return f(ctx, key, file) }
func waitRecording(t *testing.T, u *UseCase, id, state string) model.Recording {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rows, _, e := u.Recordings(context.Background(), "", 100)
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range rows {
			if v.ID == id && v.State == state && v.Attempts > 0 {
				return v
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("recording did not reach %s", state)
	return model.Recording{}
}
func TestUploadRetrySurvivesRestart(t *testing.T) {
	u, r := fixture(t, fakeMedia{})
	file := filepath.Join(u.cfg.DataDir, "clip.mp4")
	if e := os.WriteFile(file, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	rec := model.Recording{ID: "upload1", RoundID: "round1", ChannelID: "lapak01", State: "READY", File: file, Key: "rounds/r1/a.mp4"}
	if e := r.Save(context.Background(), rec); e != nil {
		t.Fatal(e)
	}
	u.storage = uploadFunc(func(context.Context, string, string) error { return errors.New("offline") })
	if e := u.Start(); e != nil {
		t.Fatal(e)
	}
	failed := waitRecording(t, u, "upload1", "READY")
	u.Close()
	if failed.Attempts != 1 || !failed.NextAttempt.After(time.Now()) {
		t.Fatalf("retry not persisted: %+v", failed)
	}
	failed.NextAttempt = time.Now().Add(-time.Second)
	if e := r.Save(context.Background(), failed); e != nil {
		t.Fatal(e)
	}
	next := New(u.cfg, u.sources, r, fakeMedia{}, uploadFunc(func(_ context.Context, key, path string) error {
		if key != rec.Key || path != file {
			return errors.New("unstable upload identity")
		}
		return nil
	}), u.disk, worker.New())
	if e := next.Start(); e != nil {
		t.Fatal(e)
	}
	defer next.Close()
	uploaded := waitRecording(t, next, "upload1", "UPLOADED")
	if uploaded.Attempts != 2 {
		t.Fatal(uploaded)
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("local recording was deleted", e)
	}
}

type failGuardExecutor struct {
	*worker.Worker
	entered <-chan struct{}
	err     error
}

func (e *failGuardExecutor) RunGuard(func(context.Context)) error {
	select {
	case <-e.entered:
		return e.err
	case <-time.After(2 * time.Second):
		return errors.New("upload did not start")
	}
}
func TestStartupFailureCancelsAlreadyRunningUpload(t *testing.T) {
	u, r := fixture(t, fakeMedia{})
	file := filepath.Join(u.cfg.DataDir, "clip.mp4")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := r.Save(context.Background(), model.Recording{ID: "startup", RoundID: "r1", ChannelID: "lapak01", State: "READY", File: file, Key: "stable.mp4"}); err != nil {
		t.Fatal(err)
	}
	entered, exited := make(chan struct{}), make(chan struct{})
	u.storage = uploadFunc(func(ctx context.Context, _, _ string) error {
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	})
	want := errors.New("guard startup failed")
	u.executor = &failGuardExecutor{Worker: u.executor.(*worker.Worker), entered: entered, err: want}
	defer u.Close()
	if err := u.Start(); !errors.Is(err, want) {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("failed startup left upload running")
	}
	if u.executor.Context().Err() == nil {
		t.Fatal("worker context not canceled")
	}
	if _, err := u.StartRound(context.Background(), model.StartRound{ID: "new", Channels: []string{"lapak01"}}); !errors.Is(err, apperror.ErrUnavailable) {
		t.Fatal(err)
	}
	rows, err := r.List(context.Background())
	if err != nil || len(rows) != 1 || rows[0].State != "READY" || rows[0].Attempts != 0 {
		t.Fatalf("canceled upload changed queue: %+v %v", rows, err)
	}
}
