package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	input "kicau.cloud/control/internal/judging/model"
)

func (u *UseCase) assignment(ctx context.Context, actor, round string) (model.Record, error) {
	if !model.UUID(round) {
		return nil, apperror.ErrInvalid
	}
	row, e := u.repo.Round(ctx, round)
	if e != nil {
		return nil, e
	}
	allowed, e := u.auth.HasArenaRole(ctx, actor, model.Text(row, "arena_id"), "juri")
	if e != nil {
		return nil, e
	}
	if !allowed {
		return nil, apperror.ErrForbidden
	}
	return u.repo.Assignment(ctx, round, actor)
}
func (u *UseCase) Rounds(ctx context.Context, actor, cursor string, limit int) ([]model.Record, error) {
	if limit < 1 || limit > 100 {
		return nil, apperror.ErrInvalid
	}
	return u.repo.Rounds(ctx, actor, cursor, limit+1)
}
func (u *UseCase) Participants(ctx context.Context, actor, round string) ([]model.Record, error) {
	if _, e := u.assignment(ctx, actor, round); e != nil {
		return nil, e
	}
	return u.repo.Items(ctx, round)
}
func (u *UseCase) read(ctx context.Context, item, judge string) (model.Record, error) {
	row, e := u.repo.Assessment(ctx, item, judge)
	if errors.Is(e, apperror.ErrNotFound) {
		row = model.Record{"version": 0, "scores": []model.Record{}}
		return row, nil
	}
	if e != nil {
		return nil, e
	}
	scores, e := u.repo.Scores(ctx, item, judge)
	if e != nil {
		return nil, e
	}
	row["scores"] = scores
	return row, nil
}
func (u *UseCase) Assessment(ctx context.Context, actor, round, item string) (model.Record, error) {
	if !model.UUID(item) {
		return nil, apperror.ErrInvalid
	}
	j, e := u.assignment(ctx, actor, round)
	if e != nil {
		return nil, e
	}
	it, e := u.repo.Item(ctx, item)
	if e != nil {
		return nil, e
	}
	if model.Text(it, "round_id") != round || model.Text(it, "status") != "CONFIRMED" {
		return nil, apperror.ErrNotFound
	}
	row, e := u.read(ctx, item, model.Text(j, "id"))
	if e != nil {
		return nil, e
	}
	criteria, e := u.repo.Criteria(ctx, model.Text(it, "arena_id"))
	if e != nil {
		return nil, e
	}
	row["criteria"] = criteria
	return row, nil
}
func (u *UseCase) History(ctx context.Context, actor, round, item string) ([]model.Record, error) {
	if _, e := u.Assessment(ctx, actor, round, item); e != nil {
		return nil, e
	}
	j, e := u.assignment(ctx, actor, round)
	if e != nil {
		return nil, e
	}
	return u.repo.History(ctx, item, model.Text(j, "id"))
}
func (u *UseCase) Submit(ctx context.Context, actor, round, item string, in input.Submit, revision bool) (model.Record, error) {
	if !model.UUID(round) || !model.UUID(item) || !model.UUID(in.OperationID) || in.ExpectedVersion < 0 || len(in.Values) < 1 || len(in.Values) > 100 {
		return nil, apperror.ErrInvalid
	}
	in.Values = append([]input.Value(nil), in.Values...)
	sort.Slice(in.Values, func(i, j int) bool { return in.Values[i].CriteriaID < in.Values[j].CriteriaID })
	seen := map[string]bool{}
	for _, v := range in.Values {
		if !model.UUID(v.CriteriaID) || v.Value < 1 || v.Value > 10 || seen[v.CriteriaID] {
			return nil, apperror.ErrInvalid
		}
		seen[v.CriteriaID] = true
	}
	canonical, _ := json.Marshal(struct {
		Round, Item string
		Revision    bool
		Input       input.Submit
	}{round, item, revision, in})
	hash := sha256.Sum256(canonical)
	var out model.Record
	e := u.tx.Do(ctx, func(ctx context.Context) error {
		rr, e := u.repo.LockRound(ctx, round)
		if e != nil {
			return e
		}
		j, e := u.assignment(ctx, actor, round)
		if e != nil {
			return e
		}
		op, e := u.repo.Operation(ctx, actor, in.OperationID)
		if e == nil {
			if model.Text(op, "payload_hash") != hex.EncodeToString(hash[:]) {
				return apperror.Public(apperror.ErrConflict, "Operation ID was used with different content")
			}
			result, ok := op["result"].(map[string]any)
			if !ok {
				return apperror.ErrConflict
			}
			out = model.Record(result)
			return nil
		}
		if !errors.Is(e, apperror.ErrNotFound) {
			return e
		}
		if model.Text(rr, "phase") != "JUDGING" {
			return apperror.ErrConflict
		}
		it, e := u.repo.Item(ctx, item)
		if e != nil {
			return e
		}
		arena := model.Text(rr, "arena_id")
		if model.Text(it, "round_id") != round || model.Text(it, "arena_id") != arena || model.Text(it, "status") != "CONFIRMED" {
			return apperror.ErrNotFound
		}
		criteria, e := u.repo.Criteria(ctx, arena)
		if e != nil {
			return e
		}
		if len(criteria) != len(seen) {
			return apperror.Invalid("Submit all participant criteria together")
		}
		for _, c := range criteria {
			if !seen[model.Text(c, "id")] {
				return apperror.ErrInvalid
			}
		}
		judge := model.Text(j, "id")
		current, e := u.repo.Assessment(ctx, item, judge)
		version := 0
		if e == nil {
			version = model.Number(current, "version")
		} else if !errors.Is(e, apperror.ErrNotFound) {
			return e
		}
		if version != in.ExpectedVersion || revision && version == 0 || !revision && version != 0 {
			return apperror.Public(apperror.ErrConflict, "Assessment version changed")
		}
		version++
		parent, e := u.repo.SaveAssessment(ctx, arena, round, item, judge, version)
		if e != nil {
			return e
		}
		for _, v := range in.Values {
			s, e := u.repo.Score(ctx, arena, round, item, judge, v.CriteriaID, v.Value, version)
			if e != nil {
				return e
			}
			if e = u.repo.Revision(ctx, model.Text(s, "id"), in.OperationID, version, v.Value); e != nil {
				return e
			}
		}
		out, e = u.read(ctx, item, judge)
		if e != nil {
			return e
		}
		result, e := json.Marshal(out)
		if e != nil {
			return e
		}
		return u.repo.SaveOperation(ctx, actor, in.OperationID, hash[:], model.Text(parent, "id"), string(result))
	})
	return out, e
}
