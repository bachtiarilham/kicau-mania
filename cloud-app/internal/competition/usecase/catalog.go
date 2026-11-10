package usecase

import (
	"context"
	"time"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) Get(ctx context.Context, actor, res, id, arena string) (model.Record, error) {
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
	return row, nil
}
func (u *UseCase) List(ctx context.Context, actor, res, arena, cursor string, limit int) ([]model.Record, error) {
	if !model.UUID(arena) || limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	return u.repo.List(ctx, res, arena, cursor, limit+1)
}
func catalogValidate(res string, f model.Record, create bool) error {
	allowed, required, ids, pos, money := "", "", "", "", ""
	switch res {
	case "events":
		allowed = "arena_id title date location status"
		required = "arena_id title date location"
		ids = "arena_id"
	case "categories":
		allowed = "arena_id code name"
		required = "arena_id code name"
		ids = "arena_id"
	case "rounds":
		allowed = "arena_id event_id category_id round_number bird_type entry_fee session_duration_minutes judging_duration_minutes"
		required = "arena_id event_id category_id round_number bird_type entry_fee"
		ids = "arena_id event_id category_id"
		pos = "round_number session_duration_minutes judging_duration_minutes"
		money = "entry_fee"
	default:
		return apperror.ErrNotFound
	}
	if e := model.Strings(f, "arena_id title date location status code name event_id category_id bird_type"); e != nil {
		return e
	}
	if e := model.ValidateFields(f, allowed); e != nil {
		return e
	}
	if e := model.Validate(f, required, ids, pos, money, create); e != nil {
		return e
	}
	if d, ok := f["date"]; ok {
		if _, e := time.Parse("2006-01-02", model.Text(f, "date")); e != nil || d == nil {
			return apperror.ErrInvalid
		}
	}
	if v, ok := f["entry_fee"]; ok {
		n, e := model.Cents(v)
		if e != nil || n <= 0 {
			return apperror.Invalid("Entry fee must be positive")
		}
	}
	return nil
}
func (u *UseCase) catalogAuth(ctx context.Context, actor, arena, res string) error {
	if res == "categories" {
		return u.auth.Require(ctx, actor, arena, "admin_arena")
	}
	return u.auth.Require(ctx, actor, arena, "admin_arena", "operator")
}
func (u *UseCase) Create(ctx context.Context, actor, res, arena string, f model.Record) (model.Record, error) {
	if arena == "" {
		arena = model.Text(f, "arena_id")
	}
	if v := model.Text(f, "arena_id"); v != "" && v != arena {
		return nil, apperror.ErrInvalid
	}
	f["arena_id"] = arena
	if e := catalogValidate(res, f, true); e != nil {
		return nil, e
	}
	if e := u.catalogAuth(ctx, actor, arena, res); e != nil {
		return nil, e
	}
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		row, e := u.repo.Create(ctx, res, f)
		if e != nil {
			return e
		}
		out, e = u.repo.Get(ctx, res, model.Text(row, "id"))
		return e
	})
	return out, e
}
func (u *UseCase) Update(ctx context.Context, actor, res, id, arena string, f model.Record) (model.Record, error) {
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	if _, ok := f["arena_id"]; ok {
		return nil, apperror.Invalid("Arena cannot be changed")
	}
	if e := catalogValidate(res, f, false); e != nil {
		return nil, e
	}
	row, e := u.Get(ctx, actor, res, id, arena)
	if e != nil {
		return nil, e
	}
	arena = model.Text(row, "arena_id")
	if e = u.catalogAuth(ctx, actor, arena, res); e != nil {
		return nil, e
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) error {
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		if res == "rounds" {
			rr, e := u.repo.LockRound(ctx, id)
			if e != nil {
				return e
			}
			if model.Text(rr, "phase") != "PREPARED" {
				return apperror.ErrConflict
			}
			items, e := u.repo.AllItems(ctx, id)
			if e != nil {
				return e
			}
			if len(items) > 0 {
				for _, k := range []string{"event_id", "category_id", "bird_type"} {
					if _, ok := f[k]; ok {
						return apperror.ErrConflict
					}
				}
			}
		}
		if _, e := u.repo.Update(ctx, res, id, f); e != nil {
			return e
		}
		out, e = u.repo.Get(ctx, res, id)
		return e
	})
	return out, e
}
func (u *UseCase) Delete(ctx context.Context, actor, res, id, arena string) error {
	row, e := u.Get(ctx, actor, res, id, arena)
	if e != nil {
		return e
	}
	arena = model.Text(row, "arena_id")
	if e = u.catalogAuth(ctx, actor, arena, res); e != nil {
		return e
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		if res == "rounds" {
			rr, e := u.repo.LockRound(ctx, id)
			if e != nil {
				return e
			}
			if model.Text(rr, "phase") != "PREPARED" {
				return apperror.ErrConflict
			}
		}
		return u.repo.Delete(ctx, res, id)
	})
}
