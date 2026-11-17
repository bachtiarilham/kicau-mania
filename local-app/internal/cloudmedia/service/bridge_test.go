package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"kicau.local/edge/internal/mediaengine/saving/model"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnmappedRecordingStaysLocal(t *testing.T) {
	b, e := New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	e = b.PutRecording(context.Background(), model.Recording{ID: "recording01", RoundID: "session01"})
	if !errors.Is(e, model.ErrNotMapped) || b.Progress("recording01").State != "WAITING_MAPPING" {
		t.Fatal("unmapped upload admitted", e)
	}
}

func TestMKVWaitsForCloudContainerSupport(t *testing.T) {
	b, _ := New(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/uploads") || r.Method != "POST" {
			t.Error("storage transfer must not start against an old cloud contract")
		}
		var input map[string]any
		_ = json.NewDecoder(r.Body).Decode(&input)
		if input["container"] != "mkv" {
			t.Error("MKV container missing")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"id": "upload01", "object_key": "old/file.invalid", "state": "UPLOADING", "part_size": 16 << 20}})
	}))
	defer server.Close()
	if e := b.Configure(Pairing{Base: server.URL, Token: strings.Repeat("a", 64), EngineID: "engine01", ArenaID: "arena01"}); e != nil {
		t.Fatal(e)
	}
	if e := b.Bind(Ingestion{ID: "ingestion01", EngineID: "engine01", SessionID: "session01", RoundID: "round01", Mappings: []Mapping{{ChannelID: "g01", RegistrationID: "ticket01", GantanganID: "gantangan01"}}}); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "recording.mkv")
	if e := os.WriteFile(file, []byte("fixture-mkv"), 0600); e != nil {
		t.Fatal(e)
	}
	e := b.PutRecording(context.Background(), model.Recording{ID: "recording01", RoundID: "session01", ChannelID: "g01", File: file})
	if e == nil || !strings.Contains(b.Progress("recording01").Error, "Cloud belum mendukung MKV") {
		t.Fatal("old cloud accepted MKV with unsupported format", e)
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("local file lost", e)
	}
}
func TestDurableUploadAndMetadataRetryAfterDesktopCloses(t *testing.T) {
	dir := t.TempDir()
	b, _ := New(dir)
	data := []byte("mkv fixture ready for upload")
	file := filepath.Join(t.TempDir(), "final.mkv")
	_ = os.WriteFile(file, data, 0600)
	puts, completes := 0, 0
	uploaded := false
	var endpoint string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			if r.Header.Get("Authorization") != "" {
				t.Error("engine credential forwarded to storage")
			}
			got, _ := io.ReadAll(r.Body)
			sum := sha256.Sum256(got)
			if r.Header.Get("X-Amz-Checksum-Sha256") != base64.StdEncoding.EncodeToString(sum[:]) {
				t.Error("part checksum missing")
			}
			puts++
			w.WriteHeader(200)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("a", 64) {
			t.Error("engine credential missing")
		}
		w.Header().Set("Content-Type", "application/json")
		out := any(map[string]any{})
		switch {
		case strings.HasSuffix(r.URL.Path, "/uploads"):
			state := "UPLOADING"
			if uploaded {
				state = "UPLOADED"
			}
			out = map[string]any{"id": "upload01", "object_key": "test/final.mkv", "state": state, "part_size": 16 << 20, "parts": []any{}}
		case strings.HasSuffix(r.URL.Path, "/parts"):
			var part map[string]any
			_ = json.NewDecoder(r.Body).Decode(&part)
			out = map[string]any{"url": endpoint + "/object?signature=fixture", "headers": map[string]any{"X-Amz-Checksum-Sha256": part["sha256"]}}
		case strings.HasSuffix(r.URL.Path, "/complete"):
			completes++
			uploaded = true
			w.WriteHeader(503)
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": out})
	}))
	defer server.Close()
	endpoint = server.URL
	if e := b.Configure(Pairing{Base: endpoint, Token: strings.Repeat("a", 64), EngineID: "engine01", ArenaID: "arena01"}); e != nil {
		t.Fatal(e)
	}
	if e := b.Bind(Ingestion{ID: "ingestion01", EngineID: "engine01", SessionID: "session01", RoundID: "round01", Mappings: []Mapping{{ChannelID: "g01", RegistrationID: "ticket01", GantanganID: "gantangan01"}}}); e != nil {
		t.Fatal(e)
	}
	r := model.Recording{ID: "recording01", RoundID: "session01", ChannelID: "g01", File: file, State: "READY", StartedAt: time.Now().Add(-time.Minute), EndedAt: time.Now()}
	if e := b.PutRecording(context.Background(), r); e == nil {
		t.Fatal("metadata outage claimed success")
	}
	if p := b.Progress(r.ID); p.State != "RETRYING" || p.Bytes != int64(len(data)) {
		t.Fatal("progress lost", p)
	}
	restarted, _ := New(dir)
	if e := restarted.PutRecording(context.Background(), r); e != nil {
		t.Fatal("restart recovery failed", e)
	}
	if puts != 1 || completes != 1 || restarted.Progress(r.ID).State != "UPLOADED" {
		t.Fatal("retry reuploaded or completion lost")
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("local recording deleted")
	}
}
