package usecase

import (
	"context"
	"sort"
	"strings"

	"kicau.cloud/control/internal/apperror"
	input "kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) headerAccess(ctx context.Context, actor string, row model.Record) error {
	s, e := u.auth.Scope(ctx, actor, model.Text(row, "arena_id"))
	if e != nil {
		return e
	}
	if s.Super || s.Staff || model.Text(row, "registered_by_user_id") == actor || model.Text(row, "created_by_user_id") == actor {
		return nil
	}
	return apperror.ErrNotFound
}
func (u *UseCase) Registration(ctx context.Context, actor, id string) (model.Record, error) {
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	row, e := u.repo.Header(ctx, id)
	if e != nil {
		return nil, e
	}
	if e = u.headerAccess(ctx, actor, row); e != nil {
		return nil, e
	}
	items, e := u.repo.HeaderItems(ctx, id)
	if e != nil {
		return nil, e
	}
	payments, e := u.repo.Payments(ctx, id)
	if e != nil {
		return nil, e
	}
	row["items"], row["payments"] = items, payments
	return row, nil
}
func (u *UseCase) Registrations(ctx context.Context, actor, arena, cursor string, limit int) ([]model.Record, error) {
	if limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	s, e := u.auth.Scope(ctx, actor, arena)
	if e != nil {
		return nil, e
	}
	return u.repo.Registrations(ctx, cursor, arena, actor, s.Super || s.Staff, limit+1)
}
func (u *UseCase) registrationInput(ctx context.Context, actor string, in *input.Registration) error {
	if !model.UUID(in.ArenaID) || len(in.Items) < 1 || len(in.Items) > 100 {
		return apperror.ErrInvalid
	}
	if in.Source == "" {
		in.Source = "ONLINE"
	}
	if in.Source != "ONLINE" && in.Source != "MANUAL" {
		return apperror.ErrInvalid
	}
	if in.Source == "MANUAL" {
		if e := u.auth.Require(ctx, actor, in.ArenaID, "admin_arena", "operator"); e != nil {
			return e
		}
		if in.RegisteredBy != "" && !model.UUID(in.RegisteredBy) {
			return apperror.ErrInvalid
		}
	} else {
		in.RegisteredBy = actor
	}
	if in.Name == "" || in.Phone == "" {
		p, e := u.repo.Registrant(ctx, actor)
		if e != nil {
			return e
		}
		if in.Name == "" {
			in.Name = model.Text(p, "name")
		}
		if in.Phone == "" {
			in.Phone = model.Text(p, "phone")
		}
	}
	if strings.TrimSpace(in.Name) == "" || len(in.Name) > 150 || !model.Phone(in.Phone) {
		return apperror.ErrInvalid
	}
	seen := map[string]bool{}
	for _, it := range in.Items {
		if !model.UUID(it.RoundID) || !model.UUID(it.BirdID) || seen[it.RoundID+it.BirdID] {
			return apperror.ErrInvalid
		}
		seen[it.RoundID+it.BirdID] = true
	}
	return nil
}
func (u *UseCase) entries(ctx context.Context, arena string, items []input.Entry) (map[string]model.Record, int64, error) {
	rounds := map[string]model.Record{}
	ids := []string{}
	for _, it := range items {
		if _, ok := rounds[it.RoundID]; !ok {
			rounds[it.RoundID] = nil
			ids = append(ids, it.RoundID)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		rr, e := u.repo.LockRound(ctx, id)
		if e != nil {
			return nil, 0, e
		}
		if model.Text(rr, "arena_id") != arena || model.Text(rr, "phase") != "PREPARED" {
			return nil, 0, apperror.ErrConflict
		}
		rounds[id] = rr
	}
	var total int64
	for _, it := range items {
		bird, e := u.repo.Bird(ctx, it.BirdID)
		if e != nil {
			return nil, 0, e
		}
		rr := rounds[it.RoundID]
		if !strings.EqualFold(strings.TrimSpace(model.Text(bird, "bird_type")), strings.TrimSpace(model.Text(rr, "bird_type"))) {
			return nil, 0, apperror.Invalid("Bird type does not match round")
		}
		fee, e := model.Cents(rr["entry_fee"])
		if e != nil || fee <= 0 {
			return nil, 0, apperror.Invalid("Round fee is not configured")
		}
		total += fee
		if total > 999999999999 {
			return nil, 0, apperror.ErrInvalid
		}
	}
	return rounds, total, nil
}
func (u *UseCase) CreateRegistration(ctx context.Context, actor string, in input.Registration) (model.Record, error) {
	if e := u.registrationInput(ctx, actor, &in); e != nil {
		return nil, e
	}
	var id string
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.Registration(ctx, actor, id)
			}
		}()
		arena, e := u.repo.LockArena(ctx, in.ArenaID)
		if e != nil {
			return e
		}
		if !model.Flag(arena, "is_active") {
			return apperror.ErrConflict
		}
		rounds, total, e := u.entries(ctx, in.ArenaID, in.Items)
		if e != nil {
			return e
		}
		var user any
		if in.RegisteredBy != "" {
			user = in.RegisteredBy
		}
		row, e := u.repo.CreateHeader(ctx, in.ArenaID, user, actor, in.Source, in.Name, in.Phone)
		if e != nil {
			return e
		}
		id = model.Text(row, "id")
		for _, it := range in.Items {
			if e = u.repo.InsertItem(ctx, in.ArenaID, id, it.RoundID, it.BirdID, rounds[it.RoundID]["entry_fee"]); e != nil {
				return e
			}
		}
		return u.repo.SetTotal(ctx, id, model.Amount(total), in.Name, in.Phone)
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
func (u *UseCase) UpdateRegistration(ctx context.Context, actor, id string, in input.Registration) (model.Record, error) {
	row, e := u.Registration(ctx, actor, id)
	if e != nil {
		return nil, e
	}
	arena := model.Text(row, "arena_id")
	if in.ArenaID != "" && in.ArenaID != arena {
		return nil, apperror.ErrInvalid
	}
	if in.Name == "" {
		in.Name = model.Text(row, "registrant_name")
	}
	if in.Phone == "" {
		in.Phone = model.Text(row, "registrant_phone")
	}
	in.ArenaID = arena
	in.Source = model.Text(row, "registration_source")
	in.RegisteredBy = model.Text(row, "registered_by_user_id")
	if e = u.registrationInput(ctx, actor, &in); e != nil {
		return nil, e
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.Registration(ctx, actor, id)
			}
		}()
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		rr, e := u.repo.LockHeader(ctx, id)
		if e != nil {
			return e
		}
		if model.Text(rr, "status") != "PENDING_PAYMENT" {
			return apperror.ErrConflict
		}
		payments, e := u.repo.Payments(ctx, id)
		if e != nil {
			return e
		}
		if len(payments) > 0 {
			return apperror.Public(apperror.ErrConflict, "Registration cannot change after a payment was submitted")
		}
		old, e := u.repo.HeaderItems(ctx, id)
		if e != nil {
			return e
		}
		for _, item := range old {
			r, e := u.repo.LockRound(ctx, model.Text(item, "round_id"))
			if e != nil {
				return e
			}
			if model.Text(r, "phase") != "PREPARED" {
				return apperror.ErrConflict
			}
		}
		rounds, total, e := u.entries(ctx, arena, in.Items)
		if e != nil {
			return e
		}
		if e = u.repo.ClearItems(ctx, id); e != nil {
			return e
		}
		for _, it := range in.Items {
			if e = u.repo.InsertItem(ctx, arena, id, it.RoundID, it.BirdID, rounds[it.RoundID]["entry_fee"]); e != nil {
				return e
			}
		}
		return u.repo.SetTotal(ctx, id, model.Amount(total), in.Name, in.Phone)
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
func (u *UseCase) CancelRegistration(ctx context.Context, actor, id string) (model.Record, error) {
	row, e := u.Registration(ctx, actor, id)
	if e != nil {
		return nil, e
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.Registration(ctx, actor, id)
			}
		}()
		if _, e := u.repo.LockArena(ctx, model.Text(row, "arena_id")); e != nil {
			return e
		}
		rr, e := u.repo.LockHeader(ctx, id)
		if e != nil {
			return e
		}
		status := model.Text(rr, "status")
		if status == "CANCELLED" {
			return nil
		}
		if status != "PENDING_PAYMENT" && status != "DRAFT" {
			return apperror.ErrConflict
		}
		if e = u.repo.CancelItems(ctx, id); e != nil {
			return e
		}
		if e = u.repo.CancelPayments(ctx, id); e != nil {
			return e
		}
		return u.repo.CancelHeader(ctx, id)
	})
	if e != nil {
		return nil, e
	}
	u.notify.Notify()
	return out, nil
}

func (u *UseCase) Recordings(ctx context.Context, actor, id string) ([]model.Record, error) {
	if _, e := u.Registration(ctx, actor, id); e != nil {
		return nil, e
	}
	return u.repo.Recordings(ctx, id)
}
