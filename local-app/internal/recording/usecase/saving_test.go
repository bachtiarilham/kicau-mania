package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kicau.local/edge/internal/recording/model"
	"kicau.local/edge/internal/recording/repository"
	"kicau.local/edge/internal/recording/worker"
)

func savingFixture(t *testing.T) (*SavingUseCase, *repository.Files) {
	t.Helper()
	r, err := repository.NewFiles(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return NewSavingUseCase(r, nil, worker.NewUploadWorker()), r
}

type uploadFunc func(context.Context, string, string) error

func (f uploadFunc) Put(ctx context.Context, key, file string) error {
	return f(ctx, key, file)
}

func waitRecording(t *testing.T, u *SavingUseCase, id, state string) model.Recording {
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
	u, r := savingFixture(t)
	file := filepath.Join(t.TempDir(), "clip.mkv")
	if e := os.WriteFile(file, []byte("fixture"), 0600); e != nil {
		t.Fatal(e)
	}
	rec := model.Recording{
		ID:        "first",
		RoundID:   "r1",
		ChannelID: "lapak01",
		File:      file,
		Key:       "rounds/r1/lapak01/first.mkv",
		State:     "READY",
	}
	if e := r.Save(context.Background(), rec); e != nil {
		t.Fatal(e)
	}
	u.storage = uploadFunc(func(context.Context, string, string) error {
		return errors.New("temporary failure")
	})
	if e := u.Start(); e != nil {
		t.Fatal(e)
	}
	failed := waitRecording(t, u, "first", "READY")
	if failed.Attempts != 1 || failed.NextAttempt.Before(time.Now()) {
		t.Fatalf("expected delayed retry: %+v", failed)
	}
	u.Close()

	restarted := NewSavingUseCase(r, uploadFunc(func(context.Context, string, string) error {
		return nil
	}), worker.NewUploadWorker())
	if e := restarted.RetryRecording(context.Background(), "first"); e != nil {
		t.Fatal(e)
	}
	if e := restarted.Start(); e != nil {
		t.Fatal(e)
	}
	defer restarted.Close()

	uploaded := waitRecording(t, restarted, "first", "UPLOADED")
	if uploaded.Attempts != 2 {
		t.Fatalf("attempts should increment: %+v", uploaded)
	}
}
