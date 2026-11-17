package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"kicau.cloud/control/internal/apperror"
	core "kicau.cloud/control/internal/core/model"
	media "kicau.cloud/control/internal/media/model"
	uc "kicau.cloud/control/internal/media/usecase"
	"strings"
	"testing"
	"time"
)

type fixtureObjects struct {
	present bool
	upload  string
	starts  int
}

func (o *fixtureObjects) Begin(context.Context, string, string) (string, error) {
	o.starts++
	o.upload = "multipart-fixture"
	return o.upload, nil
}
func (o *fixtureObjects) Sign(context.Context, string, string, media.SignPart) (media.Signed, error) {
	return media.Signed{URL: "https://storage.example.test/signed-part", Headers: map[string]string{}}, nil
}
func (o *fixtureObjects) Complete(context.Context, string, string, []media.Part) error {
	o.present = true
	return nil
}
func (o *fixtureObjects) Exists(context.Context, string, int64, string) (bool, error) {
	return o.present, nil
}
func (o *fixtureObjects) Parts(_ context.Context, _ string, id string) ([]media.Part, error) {
	if id != o.upload {
		return nil, apperror.ErrNotFound
	}
	return []media.Part{{Number: 1, ETag: "fixture", SHA256: "fixture"}}, nil
}
func (o *fixtureObjects) Download(context.Context, string) (string, error) {
	return "https://storage.example.test/private-download", nil
}
func testDesktopMedia(t *testing.T, c *Container, call func(string, string, any, int) map[string]any, token *string, arena, round, ticket, gantangan string) {
	t.Helper()
	ctx := context.Background()
	objects := &fixtureObjects{}
	business := c.Handlers.Media.UC.(*uc.UseCase)
	business.Objects = objects
	actorToken := *token
	engineID, _ := core.NewID()
	pair := call("POST", "/media/engines/"+engineID, map[string]any{"arena_id": arena}, 200)
	engineToken := pair["token"].(string)
	badArena, _ := core.NewID()
	call("POST", "/media/engines/"+badArena, map[string]any{"arena_id": badArena}, 403)
	sessionID := "desktop-session-fixture"
	mapping := map[string]any{"engine_id": engineID, "session_id": sessionID, "round_id": round, "mappings": []map[string]any{{"channel_id": "gantangan01", "registration_id": ticket, "gantangan_id": gantangan}}}
	ingestion := call("POST", "/media/ingestions", mapping, 200)
	again := call("POST", "/media/ingestions", mapping, 200)
	if ingestion["id"] != again["id"] {
		t.Fatal("duplicate ingestion")
	}
	changed := map[string]any{}
	for k, v := range mapping {
		changed[k] = v
	}
	changed["mappings"] = []map[string]any{{"channel_id": "gantangan02", "registration_id": ticket, "gantangan_id": gantangan}}
	call("POST", "/media/ingestions", changed, 409)
	*token = engineToken
	call("GET", "/auth/me", nil, 401)
	digest := sha256.Sum256([]byte("recording fixture"))
	payload := map[string]any{"session_id": sessionID, "channel_id": "gantangan01", "recording_id": "recording-fixture", "size_bytes": 17, "sha256": hex.EncodeToString(digest[:]), "started_at": time.Now().Add(-time.Minute).UTC(), "ended_at": time.Now().UTC()}
	asset := call("POST", "/engine/media/uploads", payload, 200)
	id := asset["id"].(string)
	again = call("POST", "/engine/media/uploads", payload, 200)
	if again["id"] != id || objects.starts != 1 {
		t.Fatal("upload not idempotent")
	}
	bad := map[string]any{}
	for k, v := range payload {
		bad[k] = v
	}
	bad["size_bytes"] = 18
	call("POST", "/engine/media/uploads", bad, 409)
	call("POST", "/engine/media/uploads/"+id+"/parts", map[string]any{"number": 2, "sha256": "fixture"}, 400)
	call("POST", "/engine/media/uploads/"+id+"/parts", map[string]any{"number": 1, "sha256": "fixture"}, 200)
	done := call("POST", "/engine/media/uploads/"+id+"/complete", map[string]any{}, 200)
	if done["state"] != "UPLOADED" {
		t.Fatal("completion not persisted")
	}
	call("POST", "/engine/media/uploads/"+id+"/complete", map[string]any{}, 200)
	*token = actorToken
	page := call("GET", "/media/recordings?arena_id="+arena, nil, 200)
	if len(page["items"].([]any)) != 1 {
		t.Fatal("recording list missing asset")
	}
	read := call("GET", "/media/recordings/"+id, nil, 200)
	if read["multipart_id"] != nil || read["registration_id"] != ticket {
		t.Fatal("unsafe/mismatched recording")
	}
	call("GET", "/media/recordings/"+id+"/download", nil, 200)
	payload["recording_id"] = "recording-second-session"
	objects.present = false
	*token = engineToken
	second := call("POST", "/engine/media/uploads", payload, 200)
	if second["id"] == id {
		t.Fatal("second recording overwrote first asset")
	}
	*token = actorToken
	call("DELETE", "/media/engines/"+engineID, nil, 200)
	*token = engineToken
	call("GET", "/engine/media/uploads/"+id, nil, 401)
	*token = actorToken
	rotated := call("POST", "/media/engines/"+engineID, map[string]any{"arena_id": arena}, 200)
	*token = rotated["token"].(string)
	call("GET", "/engine/media/uploads/"+id, nil, 200)
	*token = engineToken
	call("GET", "/engine/media/uploads/"+id, nil, 401)
	*token = actorToken
	var count int
	var stored string
	if e := c.Database.QueryRow(ctx, "SELECT count(*),min(token_hash) FROM media.ingest_engines WHERE id=$1", engineID).Scan(&count, &stored); e != nil || count != 1 || strings.Contains(stored, engineToken) {
		t.Fatal("unsafe engine credential", e)
	}
}
