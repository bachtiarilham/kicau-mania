package repository

import (
	"context"

	"kicau.cloud/control/internal/core/model"
)

func (r *Repository) ScopedList(ctx context.Context, resource string, s model.Scope, cursor string, limit int) ([]model.Record, error) {
	switch resource {
	case "arenas":
		return r.db.Many(ctx, "ArenaCatalog", cursor, s.UserID, s.Super, limit)
	case "birds":
		return r.db.Many(ctx, "OwnerBirds", cursor, s.UserID, s.ArenaID, s.Super, s.Staff, limit)
	default:
		return r.db.Many(ctx, "ManageableOwners", cursor, s.UserID, s.ArenaID, s.Super, s.Staff, limit)
	}
}
func (r *Repository) Lookup(ctx context.Context, ring string) (model.Record, error) {
	return r.db.One(ctx, "BirdLookup", ring)
}
func (r *Repository) LockBird(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockBird", id)
}
func (r *Repository) Transfer(ctx context.Context, id, from, to, actor, reason string) error {
	if err := r.db.Exec(ctx, "TransferBird", id, from, to, actor, reason); err != nil {
		return err
	}
	return r.db.Exec(ctx, "UpdateBirdOwner", id, to)
}

func (r *Repository) LockOwner(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockOwner", id)
}
func (r *Repository) LockArena(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockArena", id)
}
func (r *Repository) ActiveRound(ctx context.Context, id string) (bool, error) {
	row, e := r.db.One(ctx, "ArenaHasActiveRound", id)
	return model.Flag(row, "active"), e
}
