package usecase

import (
	"context"
	"strings"

	"kicau.cloud/control/internal/apperror"
	input "kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) CreatePayment(ctx context.Context, actor, reg string, in input.Payment) (model.Record, error) {
	row, e := u.Registration(ctx, actor, reg)
	if e != nil {
		return nil, e
	}
	amount, e := model.Cents(in.Amount)
	if e != nil || amount <= 0 {
		return nil, apperror.ErrInvalid
	}
	if in.Method != "CASH" && in.Method != "TRANSFER" {
		return nil, apperror.ErrInvalid
	}
	if len(in.Proof) > 2048 || strings.ContainsAny(in.Proof, "\r\n\x00") {
		return nil, apperror.ErrInvalid
	}
	if in.Method == "TRANSFER" && strings.TrimSpace(in.Proof) == "" {
		return nil, apperror.Invalid("Transfer reference is required")
	}
	arena := model.Text(row, "arena_id")
	if in.Method == "CASH" {
		if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
			return nil, e
		}
	}
	var paymentID string
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.repo.Payment(ctx, paymentID)
			}
		}()
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		h, e := u.repo.LockHeader(ctx, reg)
		if e != nil {
			return e
		}
		if model.Text(h, "status") != "PENDING_PAYMENT" {
			return apperror.ErrConflict
		}
		total, e := model.Cents(h["total_amount"])
		if e != nil {
			return e
		}
		if total != amount {
			return apperror.Invalid("Payment must cover the complete registration")
		}
		p, e := u.repo.CreatePayment(ctx, arena, reg, model.Amount(amount), in.Method, in.Proof)
		if e != nil {
			return e
		}
		paymentID = model.Text(p, "id")
		return nil
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
func (u *UseCase) VerifyPayment(ctx context.Context, actor, id string, alloc []input.Allocation) (model.Record, error) {
	if !model.UUID(id) || len(alloc) < 1 || len(alloc) > 100 {
		return nil, apperror.ErrInvalid
	}
	p, e := u.repo.Payment(ctx, id)
	if e != nil {
		return nil, e
	}
	arena, reg := model.Text(p, "arena_id"), model.Text(p, "registration_id")
	if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return nil, e
	}
	chosen := map[string]string{}
	for _, a := range alloc {
		if !model.UUID(a.ItemID) || !model.UUID(a.GantanganID) || chosen[a.ItemID] != "" {
			return nil, apperror.ErrInvalid
		}
		chosen[a.ItemID] = a.GantanganID
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.Registration(ctx, actor, reg)
			}
		}()
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		h, e := u.repo.LockHeader(ctx, reg)
		if e != nil {
			return e
		}
		payment, e := u.repo.LockPayment(ctx, id)
		if e != nil {
			return e
		}
		items, e := u.repo.HeaderItems(ctx, reg)
		if e != nil {
			return e
		}
		if len(items) != len(chosen) {
			return apperror.ErrInvalid
		}
		if model.Text(payment, "status") == "VERIFIED" {
			for _, it := range items {
				if chosen[model.Text(it, "id")] != model.Text(it, "gantangan_id") {
					return apperror.ErrConflict
				}
			}
			return nil
		}
		if model.Text(payment, "status") != "PENDING" || model.Text(h, "status") != "PENDING_PAYMENT" {
			return apperror.ErrConflict
		}
		amount, e := model.Cents(payment["amount"])
		if e != nil {
			return e
		}
		total, e := model.Cents(h["total_amount"])
		if e != nil {
			return e
		}
		if amount != total {
			return apperror.ErrConflict
		}
		for _, it := range items {
			rr, e := u.repo.LockRound(ctx, model.Text(it, "round_id"))
			if e != nil {
				return e
			}
			if model.Text(rr, "phase") != "PREPARED" {
				return apperror.ErrConflict
			}
			g := chosen[model.Text(it, "id")]
			if g == "" {
				return apperror.ErrInvalid
			}
			if _, e = u.repo.ConfirmItem(ctx, model.Text(it, "id"), reg, g); e != nil {
				return e
			}
		}
		if _, e = u.repo.VerifyPayment(ctx, id, actor); e != nil {
			return e
		}
		return u.repo.ConfirmHeader(ctx, reg)
	})
	if e != nil {
		return nil, e
	}
	u.notify.Notify()
	return out, nil
}
func (u *UseCase) RejectPayment(ctx context.Context, actor, id string) (model.Record, error) {
	if !model.UUID(id) {
		return nil, apperror.ErrInvalid
	}
	p, e := u.repo.Payment(ctx, id)
	if e != nil {
		return nil, e
	}
	arena := model.Text(p, "arena_id")
	if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return nil, e
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.repo.Payment(ctx, id)
			}
		}()
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		_, e := u.repo.RejectPayment(ctx, id)
		return e
	})
	if e != nil {
		return nil, e
	}
	return out, nil
}
