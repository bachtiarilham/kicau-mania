// Package usecase manages assets through ownership and arena-scoped permissions.
package usecase

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

type Repository interface {
	LockOwner(context.Context, string) (model.Record, error)
	LockArena(context.Context, string) (model.Record, error)
	ActiveRound(context.Context, string) (bool, error)
	Get(context.Context, string, string) (model.Record, error)
	List(context.Context, string, string, string, int) ([]model.Record, error)
	ScopedList(context.Context, string, model.Scope, string, int) ([]model.Record, error)
	Create(context.Context, string, model.Record) (model.Record, error)
	Update(context.Context, string, string, model.Record) (model.Record, error)
	Delete(context.Context, string, string) error
	Lookup(context.Context, string) (model.Record, error)
	LockBird(context.Context, string) (model.Record, error)
	Transfer(context.Context, string, string, string, string, string) error
}
type Authority interface {
	Scope(context.Context, string, string) (model.Scope, error)
	Require(context.Context, string, string, ...string) error
}
type Transactions interface {
	Do(context.Context, func(context.Context) error) error
}
type UseCase struct {
	repo Repository
	auth Authority
	tx   Transactions
}

func New(r Repository, a Authority, tx Transactions) *UseCase { return &UseCase{r, a, tx} }
func (u *UseCase) access(ctx context.Context, actor, res string, row model.Record, write bool) error {
	if res == "owners" || res == "birds" {
		owner := row
		if res == "birds" {
			var e error
			owner, e = u.repo.Get(ctx, "owners", model.Text(row, "owner_id"))
			if e != nil {
				return e
			}
		}
		scope, e := u.auth.Scope(ctx, actor, model.Text(owner, "created_in_arena_id"))
		if e != nil {
			return e
		}
		if scope.Super || model.Text(owner, "user_id") == actor || (scope.Staff && owner["user_id"] == nil) {
			return nil
		}
		return apperror.ErrNotFound
	}
	arena := model.Text(row, "arena_id")
	if res == "arenas" {
		arena = model.Text(row, "id")
	}
	if write {
		if res == "arenas" {
			return u.auth.Require(ctx, actor, arena, "admin_arena")
		}
		return u.auth.Require(ctx, actor, arena, "admin_arena")
	}
	if res == "devices" || res == "engines" {
		return u.auth.Require(ctx, actor, arena, "admin_arena", "operator")
	}
	return nil
}
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
	if e = u.access(ctx, actor, res, row, false); e != nil {
		return nil, e
	}
	return row, nil
}
func (u *UseCase) List(ctx context.Context, actor, res, arena, cursor string, limit int) ([]model.Record, error) {
	if limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	scope, e := u.auth.Scope(ctx, actor, arena)
	if e != nil {
		return nil, e
	}
	switch res {
	case "arenas", "owners", "birds":
		return u.repo.ScopedList(ctx, res, scope, cursor, limit+1)
	default:
		if !model.UUID(arena) {
			return nil, apperror.ErrInvalid
		}
		if res == "devices" || res == "engines" {
			if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
				return nil, e
			}
		}
		return u.repo.List(ctx, res, arena, cursor, limit+1)
	}
}
func validate(res string, f model.Record, create bool) error {
	allowed, required, ids, pos := "", "", "", ""
	switch res {
	case "arenas":
		allowed = "name address timezone is_active"
		required = "name"
	case "owners":
		allowed = "name phone user_id"
		required = "name"
		ids = "user_id"
	case "birds":
		allowed = "name bird_type ring_number owner_id"
		required = "name bird_type owner_id"
		ids = "owner_id"
	case "gantangan":
		allowed = "arena_id number description"
		required = "arena_id number"
		ids = "arena_id"
		pos = "number"
	case "engines":
		allowed = "arena_id name is_active"
		required = "arena_id name"
		ids = "arena_id"
	case "devices":
		allowed = "arena_id engine_id gantangan_id type ip_address rtsp_url"
		required = "arena_id engine_id type ip_address rtsp_url"
		ids = "arena_id engine_id gantangan_id"
	default:
		return apperror.ErrNotFound
	}
	if e := model.Strings(f, "arena_id user_id owner_id engine_id gantangan_id name bird_type ring_number description address timezone type ip_address rtsp_url phone"); e != nil {
		return e
	}
	if e := model.ValidateFields(f, allowed); e != nil {
		return e
	}
	if e := model.Validate(f, required, ids, pos, "", create); e != nil {
		return e
	}
	for _, k := range []string{"name", "bird_type", "ring_number", "description", "address", "timezone", "type", "ip_address", "rtsp_url", "phone"} {
		if v, ok := f[k]; ok && v != nil {
			if _, ok = v.(string); !ok {
				return apperror.ErrInvalid
			}
		}
	}
	if v, ok := f["is_active"]; ok {
		if _, ok = v.(bool); !ok {
			return apperror.ErrInvalid
		}
	}
	if zone := model.Text(f, "timezone"); zone != "" {
		if _, e := time.LoadLocation(zone); e != nil {
			return apperror.Invalid("Invalid timezone")
		}
	}
	if phone := model.Text(f, "phone"); phone != "" && !model.Phone(phone) {
		return apperror.Invalid("Phone must use E.164 format")
	}
	if ip := model.Text(f, "ip_address"); ip != "" && net.ParseIP(ip) == nil {
		return apperror.ErrInvalid
	}
	if source := model.Text(f, "rtsp_url"); source != "" {
		v, e := url.Parse(source)
		if e != nil || v.Host == "" || (v.Scheme != "rtsp" && v.Scheme != "rtsps") {
			return apperror.ErrInvalid
		}
	}
	return nil
}
func (u *UseCase) Create(ctx context.Context, actor, res, arena string, f model.Record) (model.Record, error) {
	if res == "gantangan" || res == "engines" || res == "devices" {
		if arena == "" {
			arena = model.Text(f, "arena_id")
		}
		if v := model.Text(f, "arena_id"); v != "" && v != arena {
			return nil, apperror.ErrInvalid
		}
		f["arena_id"] = arena
	}
	if e := validate(res, f, true); e != nil {
		return nil, e
	}
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		switch res {
		case "arenas":
			if e := u.auth.Require(ctx, actor, ""); e != nil {
				return e
			}
		case "owners":
			scope, e := u.auth.Scope(ctx, actor, arena)
			if e != nil {
				return e
			}
			if !scope.Super {
				if _, supplied := f["user_id"]; supplied {
					return apperror.ErrForbidden
				}
				if scope.Staff {
					f["user_id"] = nil
				} else {
					f["user_id"] = actor
				}
			}
			f["created_by_user_id"] = actor
			if scope.Staff || scope.Super && arena != "" {
				f["created_in_arena_id"] = arena
			}
		case "birds":
			owner, e := u.repo.LockOwner(ctx, model.Text(f, "owner_id"))
			if e != nil {
				return e
			}
			if e = u.access(ctx, actor, "owners", owner, true); e != nil {
				return e
			}
		default:
			if e := u.auth.Require(ctx, actor, arena, "admin_arena"); e != nil {
				return e
			}
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
	if e := validate(res, f, false); e != nil {
		return nil, e
	}
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		row, e := u.lock(ctx, res, id)
		if e != nil {
			return e
		}
		if arena != "" && model.Text(row, "arena_id") != arena {
			return apperror.ErrNotFound
		}
		if e = u.access(ctx, actor, res, row, true); e != nil {
			return e
		}
		if res == "arenas" {
			if active, ok := f["is_active"].(bool); ok && !active {
				busy, e := u.repo.ActiveRound(ctx, id)
				if e != nil {
					return e
				}
				if busy {
					return apperror.ErrConflict
				}
			}
		}
		if _, ok := f["arena_id"]; ok {
			return apperror.Invalid("Arena cannot be changed")
		}
		if res == "birds" {
			if _, ok := f["owner_id"]; ok {
				return apperror.Invalid("Use the ownership transfer action")
			}
		}
		if res == "owners" {
			if _, ok := f["user_id"]; ok {
				if e = u.auth.Require(ctx, actor, ""); e != nil {
					return e
				}
			}
		}
		if _, e = u.repo.Update(ctx, res, id, f); e != nil {
			return e
		}
		out, e = u.repo.Get(ctx, res, id)
		return e
	})
	return out, e
}
func (u *UseCase) Delete(ctx context.Context, actor, res, id, arena string) error {
	if !model.UUID(id) {
		return apperror.ErrInvalid
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		row, e := u.lock(ctx, res, id)
		if e != nil {
			return e
		}
		if arena != "" && model.Text(row, "arena_id") != arena {
			return apperror.ErrNotFound
		}
		if res == "arenas" {
			if e = u.auth.Require(ctx, actor, ""); e != nil {
				return e
			}
		} else if e = u.access(ctx, actor, res, row, true); e != nil {
			return e
		}
		return u.repo.Delete(ctx, res, id)
	})
}
func (u *UseCase) Lookup(ctx context.Context, ring string) (model.Record, error) {
	if len(strings.TrimSpace(ring)) < 1 || len(ring) > 50 {
		return nil, apperror.ErrInvalid
	}
	return u.repo.Lookup(ctx, ring)
}
func (u *UseCase) Transfer(ctx context.Context, actor, bird, to, reason string) (model.Record, error) {
	if !model.UUID(bird) || !model.UUID(to) || strings.TrimSpace(reason) == "" || len(reason) > 2000 {
		return nil, apperror.ErrInvalid
	}
	if e := u.auth.Require(ctx, actor, ""); e != nil {
		return nil, e
	}
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		b, e := u.repo.LockBird(ctx, bird)
		if e != nil {
			return e
		}
		from := model.Text(b, "owner_id")
		if from == to {
			return apperror.ErrConflict
		}
		if _, e = u.repo.Get(ctx, "owners", to); e != nil {
			return e
		}
		if e = u.repo.Transfer(ctx, bird, from, to, actor, reason); e != nil {
			return e
		}
		out, e = u.repo.Get(ctx, "birds", bird)
		return e
	})
	return out, e
}

func (u *UseCase) lock(ctx context.Context, res, id string) (model.Record, error) {
	switch res {
	case "birds":
		return u.repo.LockBird(ctx, id)
	case "owners":
		return u.repo.LockOwner(ctx, id)
	case "arenas":
		return u.repo.LockArena(ctx, id)
	default:
		return u.repo.Get(ctx, res, id)
	}
}
