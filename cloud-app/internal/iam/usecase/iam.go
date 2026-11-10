// Package usecase enforces identity, global-role and arena authorization rules.
package usecase

import (
	"context"
	"strings"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

type Repository interface {
	RemoveGlobal(context.Context, string, string) error
	Global(ctx context.Context, id string, roles []string) (model.Record, error)
	Arena(ctx context.Context, id, arena string, roles []string) (model.Record, error)
	Arenas(ctx context.Context, id string, super bool, limit int) ([]model.Record, error)
	Lookup(ctx context.Context, email string) (model.Record, error)
	GrantGlobal(ctx context.Context, id, role string) error
	GrantArena(ctx context.Context, arena, id, role string) error
	RevokeArena(ctx context.Context, id, arena string) error
	Profile(ctx context.Context, id string) (model.Record, error)
	PatchProfile(ctx context.Context, id string, fields model.Record) (model.Record, error)
	SetActive(ctx context.Context, id string, active bool) (model.Record, error)
	Users(ctx context.Context, cursor string, limit int) ([]model.Record, error)
	Roles(context.Context) ([]model.Record, error)
	Assignments(ctx context.Context, arena string) ([]model.Record, error)
}

type UseCase struct{ repo Repository }

func New(r Repository) *UseCase { return &UseCase{r} }
func (u *UseCase) Super(ctx context.Context, id string) (bool, error) {
	r, e := u.repo.Global(ctx, id, []string{"superadmin"})
	return model.Flag(r, "allowed"), e
}
func (u *UseCase) Scope(ctx context.Context, id, arena string) (model.Scope, error) {
	s := model.Scope{UserID: id, ArenaID: arena}
	var e error
	s.Super, e = u.Super(ctx, id)
	if e != nil {
		return s, e
	}
	if arena != "" {
		if !model.UUID(arena) {
			return s, apperror.ErrInvalid
		}
		r, e := u.repo.Arena(ctx, id, arena, []string{"admin_arena", "operator"})
		if e != nil {
			return s, e
		}
		s.Staff = model.Flag(r, "allowed")
	}
	return s, nil
}
func (u *UseCase) Require(ctx context.Context, id, arena string, roles ...string) error {
	super, e := u.Super(ctx, id)
	if e != nil {
		return e
	}
	if super {
		return nil
	}
	if !model.UUID(arena) {
		return apperror.ErrForbidden
	}
	r, e := u.repo.Arena(ctx, id, arena, roles)
	if e != nil {
		return e
	}
	if !model.Flag(r, "allowed") {
		return apperror.ErrForbidden
	}
	return nil
}
func (u *UseCase) Arenas(ctx context.Context, id string) ([]model.Record, error) {
	s, e := u.Super(ctx, id)
	if e != nil {
		return nil, e
	}
	return u.repo.Arenas(ctx, id, s, 101)
}
func (u *UseCase) Users(ctx context.Context, id, cursor string, limit int) ([]model.Record, error) {
	if e := u.Require(ctx, id, ""); e != nil {
		return nil, e
	}
	if limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	return u.repo.Users(ctx, cursor, limit+1)
}
func (u *UseCase) Lookup(ctx context.Context, id, arena, email string) (model.Record, error) {
	if e := u.Require(ctx, id, arena, "admin_arena"); e != nil {
		return nil, e
	}
	return u.repo.Lookup(ctx, strings.ToLower(strings.TrimSpace(email)))
}
func (u *UseCase) Roles(ctx context.Context) ([]model.Record, error) { return u.repo.Roles(ctx) }
func (u *UseCase) Assignments(ctx context.Context, id, arena string) ([]model.Record, error) {
	if e := u.Require(ctx, id, arena, "admin_arena"); e != nil {
		return nil, e
	}
	return u.repo.Assignments(ctx, arena)
}
func (u *UseCase) Grant(ctx context.Context, actor, arena, id, role string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	if arena == "" {
		if e := u.Require(ctx, actor, ""); e != nil {
			return e
		}
		switch role {
		case "superadmin", "peserta", "penjoki":
			return u.repo.GrantGlobal(ctx, id, role)
		default:
			return apperror.ErrInvalid
		}
	}
	if e := u.Require(ctx, actor, arena, "admin_arena"); e != nil {
		return e
	}
	switch role {
	case "operator", "juri":
	case "admin_arena":
		if e := u.Require(ctx, actor, ""); e != nil {
			return e
		}
	default:
		return apperror.ErrInvalid
	}
	return u.repo.GrantArena(ctx, arena, id, role)
}
func (u *UseCase) Revoke(ctx context.Context, actor, arena, id string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	if e := u.Require(ctx, actor, arena, "admin_arena"); e != nil {
		return e
	}
	rows, e := u.repo.Assignments(ctx, arena)
	if e != nil {
		return e
	}
	for _, row := range rows {
		if model.Text(row, "id") == id {
			if model.Text(row, "role") == "admin_arena" {
				if e = u.Require(ctx, actor, ""); e != nil {
					return e
				}
			}
			return u.repo.RevokeArena(ctx, id, arena)
		}
	}
	return apperror.ErrNotFound
}
func (u *UseCase) PatchProfile(ctx context.Context, id string, fields model.Record) (model.Record, error) {
	if e := model.ValidateFields(fields, "name phone"); e != nil {
		return nil, e
	}
	if name, ok := fields["name"]; ok && (name == nil || strings.TrimSpace(model.Text(fields, "name")) == "" || len(model.Text(fields, "name")) > 100) {
		return nil, apperror.ErrInvalid
	}
	if phone, ok := fields["phone"]; ok && (phone == nil || !model.Phone(model.Text(fields, "phone"))) {
		return nil, apperror.ErrInvalid
	}
	if _, e := u.repo.PatchProfile(ctx, id, fields); e != nil {
		return nil, e
	}
	return u.repo.Profile(ctx, id)
}
func (u *UseCase) SetActive(ctx context.Context, actor, id string, active bool) (model.Record, error) {
	if e := u.Require(ctx, actor, ""); e != nil {
		return nil, e
	}
	if strings.EqualFold(id, actor) && !active {
		return nil, apperror.ErrConflict
	}
	if _, e := u.repo.SetActive(ctx, id, active); e != nil {
		return nil, e
	}
	return u.repo.Profile(ctx, id)
}

func (u *UseCase) HasArenaRole(ctx context.Context, id, arena, role string) (bool, error) {
	row, e := u.repo.Arena(ctx, id, arena, []string{role})
	return model.Flag(row, "allowed"), e
}

func (u *UseCase) RemoveGlobal(ctx context.Context, actor, id, role string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	if e := u.Require(ctx, actor, ""); e != nil {
		return e
	}
	if strings.EqualFold(actor, id) && role == "superadmin" {
		return apperror.ErrConflict
	}
	switch role {
	case "superadmin", "peserta", "penjoki":
		return u.repo.RemoveGlobal(ctx, id, role)
	default:
		return apperror.ErrInvalid
	}
}
