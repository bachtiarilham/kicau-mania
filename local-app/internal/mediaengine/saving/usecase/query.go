// Recording queries provide pagination and lookup by session.
package usecase

import (
	"context"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/mediaengine/saving/model"
)

func (u *UseCase) Recordings(ctx context.Context, cursor string, limit int) ([]model.Recording, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", apperror.ErrInvalid
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	all, e := u.repo.List(ctx)
	if e != nil {
		return nil, "", e
	}
	out := []model.Recording{}
	next := ""
	for _, r := range all {
		if r.ID <= cursor {
			continue
		}
		if len(out) == limit {
			next = out[len(out)-1].ID
			break
		}
		out = append(out, r)
	}
	return out, next, nil
}

// Session resolves media recording metadata, independently of judging rounds.
func (u *UseCase) Session(ctx context.Context, id string) ([]model.Recording, error) {
	if !identifier.Valid(id) {
		return nil, apperror.ErrInvalid
	}
	all, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []model.Recording{}
	for _, r := range all {
		if r.RoundID == id {
			out = append(out, r)
		}
	}
	return out, nil
}
