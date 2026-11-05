package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"kicau.local/edge/internal/core/arena/model"
)

func TestEmbeddedHistoryMigratesWithoutLosingRevisions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "arena.sqlite")
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	event := model.Event{Bundle: model.Bundle{ID: "event"}, Revisions: []model.Revision{{OperationID: "op", RoundID: "round", JudgeID: "judge", ParticipantID: "p1", Number: 1, Fingerprint: "original-hash", Scores: map[string]int{"c1": 8}}}}
	body, _ := json.Marshal(event)
	if _, err := r.db.Exec(`INSERT INTO events(id,body) VALUES(?,?)`, "event", body); err != nil {
		t.Fatal(err)
	}
	r.Close()
	r, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got, err := r.Event(context.Background(), "event")
	if err != nil || len(got.Revisions) != 1 || got.Revisions[0].Fingerprint != "original-hash" {
		t.Fatal("migration lost history", err)
	}
	metadata, err := r.Metadata(context.Background(), "event")
	if err != nil || len(metadata.Revisions) != 0 {
		t.Fatal("history still duplicated in aggregate", err)
	}
	judge, err := r.JudgeEvent(context.Background(), "event", "judge")
	if err != nil || len(judge.Revisions) != 1 {
		t.Fatal("judge index lost migrated history", err)
	}
}
