// Package repository adapts named SQL to the workflow data port.
package repository

import (
	"context"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type Repository struct{ Store *postgres.Store }

func New(s *postgres.Store) *Repository { return &Repository{Store: s} }

func (r *Repository) One(c context.Context, n string, a ...any) (model.Record, error) {
	return r.Store.One(c, n, a...)
}
func (r *Repository) Many(c context.Context, n string, a ...any) ([]model.Record, error) {
	return r.Store.Many(c, n, a...)
}
func (r *Repository) Exec(c context.Context, n string, a ...any) error {
	return r.Store.Exec(c, n, a...)
}
func (r *Repository) Do(c context.Context, f func(context.Context) error) error {
	return r.Store.Do(c, f)
}
