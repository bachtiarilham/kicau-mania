package usecase

import (
	"context"
	"errors"
	"kicau.local/edge/internal/mediaengine/saving/model"
	"kicau.local/edge/internal/mediaengine/saving/repository"
	"kicau.local/edge/internal/mediaengine/saving/worker"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T) (*UseCase, *repository.Files) {
	t.Helper()
	r, err := repository.NewFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(r, nil, worker.New()), r
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
	u, r := fixture(t)
	file := filepath.Join(t.TempDir(), "clip.mkv")
	if e := os.WriteFile(file, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	rec := model.Recording{ID: "upload1", RoundID: "round1", ChannelID: "lapak01", State: "READY", File: file, Key: "rounds/r1/a.mkv"}
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
	next := New(r, uploadFunc(func(_ context.Context, key, path string) error {
		if key != rec.Key || path != file {
			return errors.New("unstable upload identity")
		}
		return nil
	}), worker.New())
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
