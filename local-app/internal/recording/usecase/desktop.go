package usecase

import (
	"context"
	"time"

	"kicau.local/edge/internal/apperror"
)

func (u *SavingUseCase) RecordingFile(c context.Context, id string) (string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	rows, e := u.repo.List(c)
	if e != nil {
		return "", e
	}
	for _, r := range rows {
		if r.ID == id {
			if r.State != "READY" && r.State != "UPLOADED" {
				return "", apperror.ErrConflict
			}
			return r.File, nil
		}
	}
	return "", apperror.ErrNotFound
}

func (u *SavingUseCase) RetryRecording(c context.Context, id string) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	rows, e := u.repo.List(c)
	if e != nil {
		return e
	}
	for _, r := range rows {
		if r.ID == id {
			if r.State == "UPLOADED" {
				return nil
			}
			if r.State != "READY" {
				return apperror.ErrConflict
			}
			r.NextAttempt = time.Time{}
			return u.repo.Save(c, r)
		}
	}
	return apperror.ErrNotFound
}
