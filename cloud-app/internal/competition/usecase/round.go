package usecase

import (
	"context"
	"math/big"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) RoundAction(ctx context.Context, actor, id, action, winner string) (model.Record, error) {
	row, e := u.Get(ctx, actor, "rounds", id, "")
	if e != nil {
		return nil, e
	}
	arena := model.Text(row, "arena_id")
	if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return nil, e
	}
	var out model.Record
	e = u.tx.Do(ctx, func(ctx context.Context) (resultErr error) {
		defer func() {
			if resultErr == nil {
				out, resultErr = u.repo.Get(ctx, "rounds", id)
			}
		}()
		a, e := u.repo.LockArena(ctx, arena)
		if e != nil {
			return e
		}
		rr, e := u.repo.LockRound(ctx, id)
		if e != nil {
			return e
		}
		phase := model.Text(rr, "phase")
		next := ""
		var from, win, by any
		switch action {
		case "start":
			if phase != "PREPARED" || !model.Flag(a, "is_active") {
				return apperror.ErrConflict
			}
			items, e := u.repo.Items(ctx, id)
			if e != nil {
				return e
			}
			judges, e := u.repo.Judges(ctx, id)
			if e != nil {
				return e
			}
			criteria, e := u.repo.Criteria(ctx, arena)
			if e != nil {
				return e
			}
			if len(items) == 0 || len(judges) == 0 || len(criteria) == 0 || len(criteria) > 100 {
				return apperror.Public(apperror.ErrConflict, "Participants, judges and criteria are required")
			}
			if e = u.repo.LockCriteria(ctx, arena); e != nil {
				return e
			}
			next = "RUNNING"
		case "begin-judging":
			if phase != "RUNNING" {
				return apperror.ErrConflict
			}
			next = "JUDGING"
		case "interrupt":
			if phase != "RUNNING" && phase != "JUDGING" {
				return apperror.ErrConflict
			}
			next = "INTERRUPTED"
			from = phase
		case "resume":
			if phase != "INTERRUPTED" {
				return apperror.ErrConflict
			}
			next = model.Text(rr, "interrupted_from_phase")
		case "cancel":
			if phase == "CLOSED" {
				return apperror.ErrConflict
			}
			if phase == "CANCELLED" {
				return nil
			}
			next = "CANCELLED"
		case "close":
			if phase == "CLOSED" && model.Text(rr, "winner_registration_item_id") == winner {
				return nil
			}
			if phase != "JUDGING" {
				return apperror.ErrConflict
			}
			if !model.UUID(winner) {
				return apperror.ErrInvalid
			}
			rows, e := u.repo.Recap(ctx, id)
			if e != nil {
				return e
			}
			if len(rows) == 0 {
				return apperror.ErrConflict
			}
			var maximum *big.Rat
			values := map[string]*big.Rat{}
			for _, s := range rows {
				if model.Number(s, "judge_count") == 0 || model.Number(s, "criteria_count") == 0 || model.Number(s, "score_count") != model.Number(s, "expected_score_count") {
					return apperror.Public(apperror.ErrConflict, "Required scores are incomplete")
				}
				score, ok := new(big.Rat).SetString(model.TextNumber(s, "total_score"))
				if !ok {
					return apperror.ErrConflict
				}
				values[model.Text(s, "registration_item_id")] = score
				if maximum == nil || score.Cmp(maximum) > 0 {
					maximum = score
				}
			}
			chosen := values[winner]
			if chosen == nil || chosen.Cmp(maximum) != 0 {
				return apperror.Invalid("Winner must be a highest-scoring candidate")
			}
			next = "CLOSED"
			win = winner
			by = actor
		default:
			return apperror.ErrNotFound
		}
		_, e = u.repo.Phase(ctx, id, next, from, win, by)
		return e
	})
	if e != nil {
		return nil, e
	}
	u.notify.Notify()
	return out, nil
}
func (u *UseCase) Recap(ctx context.Context, actor, id string) ([]model.Record, error) {
	row, e := u.Get(ctx, actor, "rounds", id, "")
	if e != nil {
		return nil, e
	}
	if e = u.auth.Require(ctx, actor, model.Text(row, "arena_id"), "admin_arena", "operator"); e != nil {
		return nil, e
	}
	return u.repo.Recap(ctx, id)
}
func (u *UseCase) Judges(ctx context.Context, actor, id string) ([]model.Record, error) {
	row, e := u.Get(ctx, actor, "rounds", id, "")
	if e != nil {
		return nil, e
	}
	if e = u.auth.Require(ctx, actor, model.Text(row, "arena_id"), "admin_arena", "operator"); e != nil {
		return nil, e
	}
	return u.repo.Judges(ctx, id)
}
func (u *UseCase) AssignJudge(ctx context.Context, actor, id, user string) ([]model.Record, error) {
	if !model.UUID(user) {
		return nil, apperror.ErrInvalid
	}
	row, e := u.Get(ctx, actor, "rounds", id, "")
	if e != nil {
		return nil, e
	}
	arena := model.Text(row, "arena_id")
	if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return nil, e
	}
	ok, e := u.auth.HasArenaRole(ctx, user, arena, "juri")
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, apperror.Invalid("User is not assigned as judge in this arena")
	}
	e = u.tx.Do(ctx, func(ctx context.Context) error {
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		rr, e := u.repo.LockRound(ctx, id)
		if e != nil {
			return e
		}
		if model.Text(rr, "phase") != "PREPARED" {
			return apperror.ErrConflict
		}
		_, e = u.repo.AssignJudge(ctx, arena, id, user)
		return e
	})
	if e != nil {
		return nil, e
	}
	return u.repo.Judges(ctx, id)
}
func (u *UseCase) RemoveJudge(ctx context.Context, actor, id, assignment string) error {
	if !model.UUID(assignment) {
		return apperror.ErrInvalid
	}
	row, e := u.Get(ctx, actor, "rounds", id, "")
	if e != nil {
		return e
	}
	arena := model.Text(row, "arena_id")
	if e = u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return e
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		if _, e := u.repo.LockArena(ctx, arena); e != nil {
			return e
		}
		rr, e := u.repo.LockRound(ctx, id)
		if e != nil {
			return e
		}
		if model.Text(rr, "phase") != "PREPARED" {
			return apperror.ErrConflict
		}
		_, e = u.repo.RemoveJudge(ctx, assignment, id)
		return e
	})
}
