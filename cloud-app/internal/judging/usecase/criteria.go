package usecase

import (
	"context"
	"strings"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) List(ctx context.Context, actor, res, arena, cursor string, limit int) ([]model.Record, error) {
	if res != "score_criteria" || !model.UUID(arena) || limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	return u.repo.List(ctx, res, arena, cursor, limit+1)
}
func (u *UseCase) Get(ctx context.Context, actor, res, id, arena string) (model.Record, error) {
	if res != "score_criteria" || !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	row, e := u.repo.Get(ctx, res, id)
	if e != nil {
		return nil, e
	}
	if arena != "" && model.Text(row, "arena_id") != arena {
		return nil, apperror.ErrNotFound
	}
	return row, nil
}
func (u *UseCase) criteriaWrite(ctx context.Context, actor, res, id, arena string, f model.Record, action string) (model.Record, error) {
	if res != "score_criteria" {
		return nil, apperror.ErrNotFound
	}
	if id != "" {
		if !model.UUID(id) {
			return nil, apperror.ErrInvalid
		}
		row, e := u.repo.Get(ctx, res, id)
		if e != nil {
			return nil, e
		}
		if arena != "" && model.Text(row, "arena_id") != arena {
			return nil, apperror.ErrNotFound
		}
		arena = model.Text(row, "arena_id")
	}
	if !model.UUID(arena) {
		return nil, apperror.ErrInvalid
	}
	if e := u.auth.Require(ctx, actor, arena, "admin_arena"); e != nil {
		return nil, e
	}
	if f != nil {
		if e := model.ValidateFields(f, "criteria_name"); e != nil {
			return nil, e
		}
		if strings.TrimSpace(model.Text(f, "criteria_name")) == "" || len(model.Text(f, "criteria_name")) > 100 {
			return nil, apperror.ErrInvalid
		}
	}
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		a, e := u.repo.LockArena(ctx, arena)
		if e != nil {
			return e
		}
		if a["criteria_locked_at"] != nil {
			return apperror.Public(apperror.ErrConflict, "Arena criteria are locked after first use")
		}
		if action == "delete" {
			return u.repo.Delete(ctx, res, id)
		}
		if action == "create" {
			existing, e := u.repo.Criteria(ctx, arena)
			if e != nil {
				return e
			}
			if len(existing) >= 100 {
				return apperror.Public(apperror.ErrConflict, "At most 100 criteria are supported per arena")
			}
			f["arena_id"] = arena
			row, e := u.repo.Create(ctx, res, f)
			if e != nil {
				return e
			}
			id = model.Text(row, "id")
		} else {
			if _, e := u.repo.Update(ctx, res, id, f); e != nil {
				return e
			}
		}
		out, e = u.repo.Get(ctx, res, id)
		return e
	})
	return out, e
}
func (u *UseCase) Create(ctx context.Context, actor, res, arena string, f model.Record) (model.Record, error) {
	if v := model.Text(f, "arena_id"); v != "" {
		if arena != "" && arena != v {
			return nil, apperror.ErrInvalid
		}
		arena = v
		delete(f, "arena_id")
	}
	return u.criteriaWrite(ctx, actor, res, "", arena, f, "create")
}
func (u *UseCase) Update(ctx context.Context, actor, res, id, arena string, f model.Record) (model.Record, error) {
	return u.criteriaWrite(ctx, actor, res, id, arena, f, "update")
}
func (u *UseCase) Delete(ctx context.Context, actor, res, id, arena string) error {
	_, e := u.criteriaWrite(ctx, actor, res, id, arena, nil, "delete")
	return e
}
