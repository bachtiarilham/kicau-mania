package usecase_test

import (
	"context"
	"errors"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
	"path/filepath"
	"testing"

	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/arena/repository"
	syncUC "kicau.local/edge/internal/edgetocloud/synchronization/usecase"
)

type cloudFunc func(context.Context, []exchange.Change) ([]string, error)

func (f cloudFunc) Send(ctx context.Context, changes []exchange.Change) ([]string, error) {
	return f(ctx, changes)
}
func TestOfflineOutboxRetriesOnlyAcknowledgedChanges(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "arena.sqlite")
	r, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Update(ctx, "event", func(e *model.Event) ([]exchange.Change, error) {
		e.Bundle.ID = "event"
		return []exchange.Change{{ID: "one", Payload: []byte(`{}`)}, {ID: "two", Payload: []byte(`{}`)}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	u := syncUC.New(r, cloudFunc(func(context.Context, []exchange.Change) ([]string, error) { return nil, errors.New("offline") }))
	if u.Flush(ctx) == nil {
		t.Fatal("cloud failure hidden")
	}
	r.Close()
	r, err = repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if count, _ := r.PendingCount(ctx); count != 2 {
		t.Fatal("offline changes lost at restart")
	}
	u = syncUC.New(r, cloudFunc(func(_ context.Context, changes []exchange.Change) ([]string, error) {
		if len(changes) != 2 {
			t.Fatal(changes)
		}
		return []string{"unrequested"}, nil
	}))
	if u.Flush(ctx) == nil {
		t.Fatal("foreign acknowledgment accepted")
	}
	if count, _ := r.PendingCount(ctx); count != 2 {
		t.Fatal("invalid response discarded changes")
	}
	u = syncUC.New(r, cloudFunc(func(_ context.Context, changes []exchange.Change) ([]string, error) {
		return []string{changes[0].ID}, nil
	}))
	if err := u.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	pending, _ := r.Pending(ctx, 100)
	if len(pending) != 1 || pending[0].ID != "two" {
		t.Fatal("partial acknowledgment lost pending data")
	}
	if err := u.Flush(ctx); err != nil {
		t.Fatal(err)
	}
	if count, _ := r.PendingCount(ctx); count != 0 {
		t.Fatal("acknowledgments not persisted")
	}
}
