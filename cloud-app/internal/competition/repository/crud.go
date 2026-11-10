package repository

import (
	"context"
	"encoding/json"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type Repository struct{ db *postgres.Store }

func New(db *postgres.Store) *Repository { return &Repository{db} }
func prefix(resource string) (string, error) {
	switch resource {
	case "events":
		return "EntityEvents", nil
	case "categories":
		return "EntityCategories", nil
	case "rounds":
		return "EntityRounds", nil
	}
	return "", apperror.ErrNotFound
}
func (r *Repository) Get(ctx context.Context, resource, id string) (model.Record, error) {
	p, e := prefix(resource)
	if e != nil {
		return nil, e
	}
	return r.db.One(ctx, p+"Get", id)
}
func (r *Repository) List(ctx context.Context, resource, arena, cursor string, limit int) ([]model.Record, error) {
	p, e := prefix(resource)
	if e != nil {
		return nil, e
	}
	return r.db.Many(ctx, p+"List", cursor, arena, limit)
}
func (r *Repository) Create(ctx context.Context, resource string, fields model.Record) (model.Record, error) {
	p, e := prefix(resource)
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(fields)
	if e != nil {
		return nil, e
	}
	return r.db.One(ctx, p+"Create", string(b))
}
func (r *Repository) Update(ctx context.Context, resource, id string, fields model.Record) (model.Record, error) {
	p, e := prefix(resource)
	if e != nil {
		return nil, e
	}
	b, e := json.Marshal(fields)
	if e != nil {
		return nil, e
	}
	return r.db.One(ctx, p+"Update", id, string(b))
}
func (r *Repository) Delete(ctx context.Context, resource, id string) error {
	p, e := prefix(resource)
	if e != nil {
		return e
	}
	_, e = r.db.One(ctx, p+"Delete", id)
	return e
}
