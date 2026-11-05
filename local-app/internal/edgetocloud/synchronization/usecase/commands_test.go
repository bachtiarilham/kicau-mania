package usecase_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"kicau.local/edge/internal/core/arena/repository"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
	syncUC "kicau.local/edge/internal/edgetocloud/synchronization/usecase"
)

type commandCloud struct {
	commands   []exchange.Command
	failReport bool
}

func (c *commandCloud) Commands(context.Context) ([]exchange.Command, error) { return c.commands, nil }
func (c *commandCloud) Report(_ context.Context, results []exchange.CommandResult) ([]string, error) {
	if c.failReport {
		return nil, errors.New("offline")
	}
	ids := []string{}
	for _, r := range results {
		ids = append(ids, r.ID)
	}
	return ids, nil
}

type runner struct{ calls int }

func (r *runner) Execute(context.Context, exchange.Command) error { r.calls++; return nil }
func TestCommandExpiryAndAcknowledgmentLoss(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "arena.sqlite")
	store, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	cloud := &commandCloud{failReport: true, commands: []exchange.Command{{ID: "expired", ArenaID: "arena", EngineID: "engine", Kind: "restream.start", ExpiresAt: time.Now().Add(-time.Minute), Payload: []byte(`{}`)}, {ID: "fresh", ArenaID: "arena", EngineID: "engine", Kind: "restream.stop", ExpiresAt: time.Now().Add(time.Minute), Payload: []byte(`{}`)}}}
	r := &runner{}
	u := syncUC.NewCommands(store, cloud, r, "arena", "engine")
	if u.Poll(ctx) == nil {
		t.Fatal("failed report not retained")
	}
	if r.calls != 1 {
		t.Fatal("expired command executed")
	}
	store.Close()
	store, err = repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cloud.failReport = false
	u = syncUC.NewCommands(store, cloud, r, "arena", "engine")
	if err := u.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if r.calls != 1 {
		t.Fatal("command re-executed after lost cloud acknowledgment")
	}
	results, err := store.CommandResults(ctx)
	if err != nil || len(results) != 0 {
		t.Fatal("results not acknowledged")
	}
	cloud.commands[1].Kind = "recording.start"
	if u.Poll(ctx) == nil {
		t.Fatal("command identity reused")
	}
}
