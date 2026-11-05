package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kicau.cloud/control/internal/exchange/model"
	"kicau.cloud/control/internal/exchange/repository"
	"kicau.cloud/control/internal/exchange/usecase"
)

func TestDurableAcknowledgmentAndEngineIsolation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cloud.sqlite")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	engines := []model.Engine{{ArenaID: "arena01", EngineID: "engine01", Token: strings.Repeat("a", 32)}, {ArenaID: "arena02", EngineID: "engine02", Token: strings.Repeat("b", 32)}}
	u, err := usecase.New(store, engines, strings.Repeat("o", 32))
	if err != nil {
		t.Fatal(err)
	}
	change := model.Change{ID: "op1", EventID: "event", ArenaID: "arena01", EngineID: "engine01", Kind: "score.revised", At: time.Now().UTC(), Payload: json.RawMessage(`{"revision":1}`)}
	if _, err := u.Receive(ctx, "engine02", engines[1].Token, []model.Change{change}); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("foreign arena accepted", err)
	}
	if _, err := u.Receive(ctx, "engine01", "wrong", []model.Change{change}); !errors.Is(err, model.ErrUnauthorized) {
		t.Fatal("wrong token accepted", err)
	}
	if ids, err := u.Receive(ctx, "engine01", engines[0].Token, []model.Change{change}); err != nil || len(ids) != 1 {
		t.Fatal(err)
	}
	store.Close()
	store, err = repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	u, err = usecase.New(store, engines, strings.Repeat("o", 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.Receive(ctx, "engine01", engines[0].Token, []model.Change{change}); err != nil {
		t.Fatal("retry after lost acknowledgment failed", err)
	}
	other := change
	other.ID = "op2"
	changed := change
	changed.Payload = json.RawMessage(`{"revision":999}`)
	if _, err := u.Receive(ctx, "engine01", engines[0].Token, []model.Change{other, changed}); !errors.Is(err, model.ErrConflict) {
		t.Fatal("modified duplicate accepted", err)
	}
	// The first item must roll back with the conflicting second item.
	other.Payload = json.RawMessage(`{"revision":2}`)
	if _, err := u.Receive(ctx, "engine01", engines[0].Token, []model.Change{other}); err != nil {
		t.Fatal("batch was partially committed", err)
	}
}
func TestMediaCommandsCannotMutateJudging(t *testing.T) {
	ctx := context.Background()
	store, err := repository.Open(filepath.Join(t.TempDir(), "cloud.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	token, admin := strings.Repeat("a", 32), strings.Repeat("b", 32)
	u, err := usecase.New(store, []model.Engine{{ArenaID: "arena", EngineID: "engine", Token: token}}, admin)
	if err != nil {
		t.Fatal(err)
	}
	c := model.Command{ID: "command", ArenaID: "arena", EngineID: "engine", Kind: "judging.close", Payload: json.RawMessage(`{}`), ExpiresAt: time.Now().Add(time.Minute)}
	if err := u.Enqueue(ctx, admin, c); !errors.Is(err, model.ErrInvalid) {
		t.Fatal("remote judging command accepted", err)
	}
	c.Kind = "restream.stop"
	c.Payload = json.RawMessage(`{"target":"youtube"}`)
	if err := u.Enqueue(ctx, admin, c); err != nil {
		t.Fatal(err)
	}
	if err := u.Enqueue(ctx, admin, c); err != nil {
		t.Fatal("command retry not idempotent", err)
	}
	commands, err := u.Commands(ctx, "engine", token)
	if err != nil || len(commands) != 1 {
		t.Fatal(err)
	}
	result := model.Result{ID: c.ID, State: "SUCCEEDED", At: time.Now().UTC()}
	if _, err := u.Results(ctx, "engine", token, []model.Result{result}); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Results(ctx, "engine", token, []model.Result{result}); err != nil {
		t.Fatal("result retry not idempotent", err)
	}
	commands, err = u.Commands(ctx, "engine", token)
	if err != nil || len(commands) != 0 {
		t.Fatal("executed command still pending", err)
	}
}
