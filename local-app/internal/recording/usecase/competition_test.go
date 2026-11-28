package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/recording/model"
)

func TestCompetitionReplacesHeldRecordingAndPublishesLimitedVideo(t *testing.T) {
	u, _, files, media := sessionFixture(t)
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	req := model.StartRequest{ID: "first", Channels: []string{"one"}, Competition: &model.Competition{
		ArenaID: "arena", ArenaName: "Arena Utama", EventID: "event", EventName: "Piala Walikota",
		EventDate: "2026-11-28", RoundID: "round", RoundNumber: 1, Gantangan: map[string]int{"one": 3},
	}}
	if _, err := u.StartSession(ctx, req); err != nil {
		t.Fatal(err)
	}
	awaitState(t, u, req.ID, "RUNNING")
	if _, err := u.StopSessionWithResult(ctx, req.ID, false); err != nil {
		t.Fatal(err)
	}
	awaitState(t, u, req.ID, "STOPPED")
	rows, err := files.List(ctx)
	if err != nil || len(rows) != 1 || rows[0].State != "HELD" {
		t.Fatalf("held recording: %+v %v", rows, err)
	}
	old := rows[0]
	want := filepath.Join(u.recordingRoot(), "Arena Utama_Piala Walikota_2026-11-28", "Ronde 01", "Gantangan 03.mkv")
	if old.File != want {
		t.Fatalf("path = %q, want %q", old.File, want)
	}
	u.opts.RecordingsDir = t.TempDir()
	media.finalError = errors.New("broken container")
	req.ID = "second"
	if _, err := u.StartSession(ctx, req); err != nil {
		t.Fatal(err)
	}
	awaitState(t, u, req.ID, "RUNNING")
	if _, err := u.StopSessionWithResult(ctx, req.ID, true); err != nil {
		t.Fatal(err)
	}
	awaitState(t, u, req.ID, "STOPPED")
	visible, _, err := (&SavingUseCase{repo: files}).Recordings(ctx, "", 25)
	if err != nil || len(visible) != 1 || visible[0].State != "READY" || !visible[0].VideoIssue {
		t.Fatalf("limited recording: %+v %v", visible, err)
	}
	current := visible[0]
	want = filepath.Join(filepath.Dir(old.File), "Gantangan 03 - Rekaman Terbatas.mkv")
	if current.File != want || current.DisplayFilename != filepath.Base(want) {
		t.Fatalf("wrong replacement location: %+v", current)
	}
	if _, err := os.Stat(old.File); !os.IsNotExist(err) {
		t.Fatalf("superseded file remains: %v", err)
	}
	dir, err := os.ReadDir(filepath.Dir(current.File))
	if err != nil || len(dir) != 1 {
		t.Fatalf("extra artifacts: %v, %v", dir, err)
	}
	req.ID = "third"
	if _, err := u.StartSession(ctx, req); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("completed competition restarted: %v", err)
	}
}

func TestNoCompetitionVideoPreservesPreviousFile(t *testing.T) {
	u, _, files, _ := sessionFixture(t)
	ctx := context.Background()
	root := t.TempDir()
	base := filepath.Join(root, "Gantangan 03")
	old := model.Recording{ID: "previous", RoundID: "early-stop", ChannelID: "one", State: "HELD",
		File: base + ".mkv", Key: "previous.mkv", StorageRoot: root, FinalBase: base}
	if err := os.WriteFile(old.File, []byte("previous"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := files.Save(ctx, old); err != nil {
		t.Fatal(err)
	}
	newRecording := model.Recording{ID: "new", RoundID: "complete", ChannelID: "one", State: "RECORDING",
		File: filepath.Join(root, "empty.mkv"), Key: "new.mkv", StorageRoot: root, FinalBase: base}
	if err := u.publishCompetitionRecording(ctx, newRecording, nil, true, errors.New("no data")); err != nil {
		t.Fatal(err)
	}
	other := model.Recording{ID: "available", RoundID: "complete", ChannelID: "two", State: "READY", File: filepath.Join(root, "Gantangan 04.mkv"), Key: "available.mkv"}
	if err := files.Save(ctx, other); err != nil {
		t.Fatal(err)
	}
	ready, _, err := (&SavingUseCase{repo: files}).RecordingsFiltered(ctx, "", 25, "", "", "READY")
	if err != nil || len(ready) != 1 || ready[0].ID != other.ID {
		t.Fatalf("unexpected upload candidate: %+v %v", ready, err)
	}
	all, err := files.List(ctx)
	if err != nil || len(all) != 3 || all[1].ID != newRecording.ID || all[1].State != "UNAVAILABLE" {
		t.Fatalf("missing per-camera failure: %+v %v", all, err)
	}
	if _, err := os.Stat(old.File); err != nil {
		t.Fatal(err)
	}
}

func TestCompetitionFolderResolvesNamesThatCollide(t *testing.T) {
	u, journal, _, _ := sessionFixture(t)
	if err := u.Start(); err != nil {
		t.Fatal(err)
	}
	first := model.Competition{ArenaID: "arena", ArenaName: "Arena Utama", EventID: "event-1", EventName: "Piala Walikota",
		EventDate: "2026-11-28", RoundID: "round-1", RoundNumber: 1, Gantangan: map[string]int{"one": 3}}
	second := first
	second.EventID, second.RoundID = "event-2", "round-2"
	ctx := context.Background()
	a := model.Session{StartRequest: model.StartRequest{ID: "first", Channels: []string{"one"}, Competition: &first}}
	b := model.Session{StartRequest: model.StartRequest{ID: "second", Channels: []string{"one"}, Competition: &second}}
	if err := u.prepareRecordingLocation(ctx, &a); err != nil {
		t.Fatal(err)
	}
	if err := journal.Save(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := u.prepareRecordingLocation(ctx, &b); err != nil {
		t.Fatal(err)
	}
	if b.RecordingFolder == a.RecordingFolder || filepath.Base(filepath.Dir(b.RecordingFolder)) != "Arena Utama_Piala Walikota_2026-11-28 (2)" {
		t.Fatalf("colliding event folders: %q, %q", a.RecordingFolder, b.RecordingFolder)
	}
}
