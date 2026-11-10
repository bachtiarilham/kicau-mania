// Package repository reads identities and persists role assignments.
package repository

import (
	"context"
	"encoding/json"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type Repository struct{ db *postgres.Store }

func New(db *postgres.Store) *Repository { return &Repository{db} }
func (r *Repository) Global(ctx context.Context, id string, roles []string) (model.Record, error) {
	return r.db.One(ctx, "GlobalRole", id, roles)
}
func (r *Repository) Arena(ctx context.Context, id, arena string, roles []string) (model.Record, error) {
	return r.db.One(ctx, "ArenaRole", id, arena, roles)
}
func (r *Repository) Arenas(ctx context.Context, id string, super bool, limit int) ([]model.Record, error) {
	return r.db.Many(ctx, "UserArenas", id, super, limit)
}
func (r *Repository) Lookup(ctx context.Context, email string) (model.Record, error) {
	return r.db.One(ctx, "UserLookup", email)
}
func (r *Repository) GrantGlobal(ctx context.Context, id, role string) error {
	return r.db.Exec(ctx, "AssignGlobalRole", id, role)
}
func (r *Repository) GrantArena(ctx context.Context, arena, id, role string) error {
	return r.db.Exec(ctx, "AssignArenaRole", arena, id, role)
}
func (r *Repository) RevokeArena(ctx context.Context, id, arena string) error {
	return r.db.Exec(ctx, "RevokeArenaRole", id, arena)
}
func (r *Repository) Profile(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "UserProfile", id)
}
func (r *Repository) PatchProfile(ctx context.Context, id string, fields model.Record) (model.Record, error) {
	b, e := json.Marshal(fields)
	if e != nil {
		return nil, e
	}
	return r.db.One(ctx, "PatchProfile", id, string(b))
}
func (r *Repository) SetActive(ctx context.Context, id string, active bool) (model.Record, error) {
	return r.db.One(ctx, "SetUserActive", id, active)
}
func (r *Repository) Users(ctx context.Context, cursor string, limit int) ([]model.Record, error) {
	return r.db.Many(ctx, "ListUsers", cursor, limit)
}
func (r *Repository) Roles(ctx context.Context) ([]model.Record, error) {
	return r.db.Many(ctx, "ListRoles")
}
func (r *Repository) Assignments(ctx context.Context, arena string) ([]model.Record, error) {
	return r.db.Many(ctx, "ListAssignments", arena)
}

func (r *Repository) RemoveGlobal(ctx context.Context, id, role string) error {
	_, e := r.db.One(ctx, "RemoveGlobalRole", id, role)
	return e
}
