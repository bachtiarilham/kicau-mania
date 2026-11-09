package usecase_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	cloudmodel "kicau.local/edge/internal/cloud/model"
	"kicau.local/edge/internal/cloud/repository"
	cloudUC "kicau.local/edge/internal/cloud/usecase"
)

type commandCloud struct {
	commands   []cloudmodel.Command
	failReport bool
}

func (c *commandCloud) Commands(context.Context) ([]cloudmodel.Command, error) {
	return c.commands, nil
}
func (c *commandCloud) Report(_ context.Context, results []cloudmodel.CommandResult) ([]string, error) {
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

func (r *runner) Execute(context.Context, cloudmodel.Command) error   { r.calls++; return nil }
func (r *runner) Reconcile(context.Context, cloudmodel.Command) error { return nil }

func TestCommandExpiryAndAcknowledgmentLoss(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "engine-state")
	store, err := repository.New(path)
	if err != nil {
		t.Fatal(err)
	}
	cloud := &commandCloud{failReport: true, commands: []cloudmodel.Command{{ID: "expired", ArenaID: "arena", EngineID: "engine", Kind: "media.session.start", ExpiresAt: time.Now().Add(-time.Minute), Payload: []byte(`{}`)}, {ID: "fresh", ArenaID: "arena", EngineID: "engine", Kind: "media.session.stop", ExpiresAt: time.Now().Add(time.Minute), Payload: []byte(`{}`)}}}
	r := &runner{}
	u := cloudUC.NewCommands(store, cloud, r, "arena", "engine")
	if u.Poll(ctx) == nil {
		t.Fatal("failed report not retained")
	}
	if r.calls != 1 {
		t.Fatal("expired command executed")
	}
	store, err = repository.New(path)
	if err != nil {
		t.Fatal(err)
	}
	cloud.failReport = false
	u = cloudUC.NewCommands(store, cloud, r, "arena", "engine")
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
	cloud.commands[1].Kind = "media.session.start.changed"
	if u.Poll(ctx) == nil {
		t.Fatal("command identity reused")
	}
}
