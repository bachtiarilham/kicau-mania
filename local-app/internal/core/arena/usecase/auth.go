package usecase

// This file authenticates event judges and manages sessions and login rate limits.

import (
	"context"
	"time"

	"golang.org/x/crypto/bcrypt"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/identifier"
)

type attempt struct {
	Count int
	Until time.Time
}

func (u *UseCase) allowLogin(key string) bool {
	u.loginMu.Lock()
	defer u.loginMu.Unlock()
	now := time.Now()
	if len(u.attempts) > 4096 {
		for k, v := range u.attempts {
			if now.After(v.Until) {
				delete(u.attempts, k)
			}
		}
		if len(u.attempts) > 4096 {
			return false
		}
	}
	a := u.attempts[key]
	if now.After(a.Until) {
		a = attempt{Until: now.Add(time.Minute)}
	}
	a.Count++
	u.attempts[key] = a
	return a.Count <= 10
}

func (u *UseCase) Login(ctx context.Context, eventID, username, password, client string) (string, time.Time, error) {
	if !identifier.Valid(eventID) || !identifier.Valid(username) || len(password) < 1 || len(password) > 72 {
		return "", time.Time{}, apperror.ErrUnauthorized
	}
	if !u.allowLogin("client:"+client) || !u.allowLogin(client+":"+eventID+":"+username) {
		return "", time.Time{}, apperror.Public(apperror.ErrUnavailable, "Terlalu banyak percobaan login. Tunggu satu menit.")
	}
	e, err := u.repo.Metadata(ctx, eventID)
	if err != nil {
		return "", time.Time{}, apperror.ErrUnauthorized
	}
	var judge *model.Judge
	for i := range e.Bundle.Judges {
		if e.Bundle.Judges[i].Username == username {
			judge = &e.Bundle.Judges[i]
			break
		}
	}
	passwordHash := u.dummyHash
	if judge != nil {
		passwordHash = []byte(judge.PasswordHash)
	}
	match := bcrypt.CompareHashAndPassword(passwordHash, []byte(password)) == nil
	if judge == nil || !match {
		return "", time.Time{}, apperror.ErrUnauthorized
	}
	token := operationID() + operationID()
	expires := time.Now().Add(u.opts.SessionTTL).UTC()
	err = u.repo.SaveSession(ctx, model.Session{TokenHash: hash([]byte(token)), EventID: eventID, JudgeID: judge.ID, Version: e.Bundle.Version, ExpiresAt: expires})
	return token, expires, err
}

func (u *UseCase) session(ctx context.Context, token string) (model.Session, error) {
	if len(token) != 64 {
		return model.Session{}, apperror.ErrUnauthorized
	}
	s, err := u.repo.Session(ctx, hash([]byte(token)))
	if err != nil {
		return s, err
	}
	if !time.Now().Before(s.ExpiresAt) {
		return s, apperror.ErrUnauthorized
	}
	return s, nil
}

func (u *UseCase) Logout(ctx context.Context, token string) error {
	if len(token) != 64 {
		return nil
	}
	return u.repo.DeleteSession(ctx, hash([]byte(token)))
}
