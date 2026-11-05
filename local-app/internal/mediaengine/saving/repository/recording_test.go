package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyManifestSchemaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	repo, e := NewFiles(dir)
	if e != nil {
		t.Fatal(e)
	}
	// Fixture uses the persisted schema before the domain split, including a
	// Windows file path and a scheduled retry. No migration should be necessary.
	legacy := `{"id":"abc123","round_id":"ronde-001","channel_id":"lapak01","state":"READY","file":"D:\\arena-data\\recordings\\abc123.mp4","object_key":"rounds/ronde-001/lapak01/abc123.mp4","attempts":3,"next_attempt":"2026-11-04T12:30:00+07:00","started_at":"2026-11-04T12:00:00+07:00","ended_at":"2026-11-04T12:10:00+07:00"}`
	path := filepath.Join(dir, "abc123.json")
	if e = os.WriteFile(path, []byte(legacy), 0600); e != nil {
		t.Fatal(e)
	}
	rows, e := repo.List(context.Background())
	if e != nil || len(rows) != 1 {
		t.Fatalf("legacy read: %+v %v", rows, e)
	}
	r := rows[0]
	if r.ID != "abc123" || r.State != "READY" || r.Attempts != 3 || r.File != `D:\arena-data\recordings\abc123.mp4` || r.Key != "rounds/ronde-001/lapak01/abc123.mp4" {
		t.Fatalf("legacy values changed: %+v", r)
	}
	if e = repo.Save(context.Background(), r); e != nil {
		t.Fatal(e)
	}
	saved, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var want, got map[string]any
	if e = json.Unmarshal([]byte(legacy), &want); e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(saved, &got); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted schema changed: %s", saved)
	}
}
