package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/mediaengine/media"
	streaming "kicau.local/edge/internal/streaming/model"
	"kicau.local/edge/internal/recording/model"
	"kicau.local/edge/internal/recording/repository"
	"kicau.local/edge/internal/recording/worker"
)

type fixtureMedia struct {
	calls      atomic.Int32
	finalError error
}

func (f *fixtureMedia) RunMedia(ctx context.Context, _, file string, ready func(), consume func([]byte)) error {
	f.calls.Add(1)
	if err := os.WriteFile(file, []byte("segment"), 0600); err != nil {
		return err
	}
	ready()
	consume(make([]byte, 6400))
	<-ctx.Done()
	return ctx.Err()
}

func (f *fixtureMedia) Finalize(_ context.Context, parts []string, output string) error {
	if f.finalError != nil {
		return f.finalError
	}
	if len(parts) == 0 {
		return errors.New("no parts")
	}
	return os.WriteFile(output, []byte("validated fixture"), 0600)
}

type fixtureCatalog struct{}

func (fixtureCatalog) HubRTSP(id string) (string, error) {
	if id != "one" {
		return "", apperror.ErrNotFound
	}
	return "rtsp://127.0.0.1/one", nil
}

type fixtureAudio struct{}

func (fixtureAudio) Activate([]string) error { return nil }
func (fixtureAudio) Deactivate()             {}
func (fixtureAudio) Ingest(string, []byte)   {}

type fixtureRestream struct{}

func (fixtureRestream) StartRestream(string, string) error  { return nil }
func (fixtureRestream) StopRestream(string) error           { return nil }
func (fixtureRestream) Restreams() []streaming.RestreamView { return nil }

type fixtureDisk struct{}

func (fixtureDisk) Free(string) (uint64, error) { return 1 << 40, nil }

type fixtureReset struct{}

func (fixtureReset) Restart() error { return nil }

func sessionFixture(t *testing.T) (*SessionUseCase, *repository.Journal, *repository.Files, *fixtureMedia) {
	t.Helper()
	dir := t.TempDir()
	journal, err := repository.NewJournal(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	files, err := repository.NewFiles(filepath.Join(dir, "queue"))
	if err != nil {
		t.Fatal(err)
	}
	pipeline := &fixtureMedia{}
	u := NewSessionUseCase(SessionOptions{DataDir: dir, MaxDuration: time.Hour}, journal, files, fixtureCatalog{}, pipeline, pipeline, fixtureAudio{}, fixtureRestream{}, fixtureDisk{}, worker.NewSessionWorker(), fixtureReset{})
	t.Cleanup(u.Close)
	return u, journal, files, pipeline
}

func awaitState(t *testing.T, u *SessionUseCase, id, state string) model.Session {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		s, err := u.Get(context.Background(), id)
		if err == nil && s.State == state {
			return s
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatalf("session did not reach %s", state)
	return model.Session{}
}

func TestSessionOwnsBackgroundAndPlaybackBarrier(t *testing.T) {
	u, _, files, _ := sessionFixture(t)
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	request := model.StartRequest{ID: "session1", Channels: []string{"one"}}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := u.StartSession(ctx, request); err != nil {
		t.Fatal(err)
	}
	cancel()
	awaitState(t, u, request.ID, "RUNNING")
	if _, err := u.StartSession(context.Background(), request); err != nil {
		t.Fatal("active retry", err)
	}
	if _, err := u.StartSession(context.Background(), model.StartRequest{ID: "other", Channels: request.Channels}); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal(err)
	}
	release, err := u.AcquirePlayback("one")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = u.StopSession(context.Background(), request.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = u.AcquirePlayback("one"); err == nil {
		t.Fatal("admitted after Stop")
	}
	s, _ := u.Get(context.Background(), request.ID)
	if s.State != "STOPPING" {
		t.Fatal("barrier bypassed", s.State)
	}
	release()
	awaitState(t, u, request.ID, "STOPPED")
	records, err := files.List(context.Background())
	if err != nil || len(records) != 1 || records[0].State != "READY" {
		t.Fatal(records, err)
	}
	if _, err = u.StopSession(context.Background(), request.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = u.StartSession(context.Background(), request); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("reused ID", err)
	}
}

func TestRestartFinalizesWithoutResuming(t *testing.T) {
	u, journal, files, pipeline := sessionFixture(t)
	now := time.Now().UTC()
	file := filepath.Join(u.opts.DataDir, "recordings", "recording.mkv")
	part := file + ".segment-000001.mkv.partial"
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, []byte("partial fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	s := model.Session{StartRequest: model.StartRequest{ID: "interrupted", Channels: []string{"one"}}, State: "RUNNING", StartedAt: now, ChannelsState: []model.Channel{{ID: "one", RecordingID: "recording", State: "RUNNING"}}}
	if err := journal.Save(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	rec := model.Recording{ID: "recording", RoundID: s.ID, ChannelID: "one", State: "RECORDING", File: file, Key: "rounds/interrupted/one/recording.mkv", StartedAt: now, Segments: []model.Segment{{File: part, StartedAt: now}}}
	if err := files.Save(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	got := awaitState(t, u, s.ID, "INTERRUPTED")
	if pipeline.calls.Load() != 0 || got.ChannelsState[0].RecordingState != "READY" {
		t.Fatal(got)
	}
	if _, err := u.AcquirePlayback("one"); err == nil {
		t.Fatal("reopened interrupted session")
	}
}

func TestFailedFinalizationPreservesSourceAndReason(t *testing.T) {
	u, _, files, pipeline := sessionFixture(t)
	pipeline.finalError = media.FinalizationError("recording segment codecs changed")
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := u.StartSession(context.Background(), model.StartRequest{ID: "incompatible", Channels: []string{"one"}}); err != nil {
		t.Fatal(err)
	}
	awaitState(t, u, "incompatible", "RUNNING")
	if _, err := u.StopSession(context.Background(), "incompatible"); err != nil {
		t.Fatal(err)
	}
	s := awaitState(t, u, "incompatible", "STOPPED")
	if s.ChannelsState[0].State != "FAILED" || s.ChannelsState[0].Error != "recording segment codecs changed" {
		t.Fatal(s)
	}
	records, err := files.List(context.Background())
	if err != nil || len(records) != 1 || records[0].State != "FAILED" {
		t.Fatal(records, err)
	}
	if _, err := os.Stat(records[0].Segments[0].File); err != nil {
		t.Fatal("source removed", err)
	}
}
