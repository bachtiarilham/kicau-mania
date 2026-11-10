package usecase

import (
	"context"
	"encoding/hex"
	"errors"
	"time"

	"kicau.cloud/control/internal/apperror"
	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) Login(ctx context.Context, in auth.Login) (auth.Tokens, error) {
	e, err := email(in.Email)
	if err != nil {
		return auth.Tokens{}, apperror.ErrUnauthorized
	}
	if err = u.limit("login:"+in.IP+":"+e, 6, time.Minute); err != nil {
		return auth.Tokens{}, err
	}
	r, err := u.repo.ByEmail(ctx, e)
	if errors.Is(err, apperror.ErrNotFound) {
		if _, err := u.passwords.Verify(ctx, "", in.Password); err != nil {
			return auth.Tokens{}, err
		}
		return auth.Tokens{}, apperror.ErrUnauthorized
	}
	if err != nil {
		return auth.Tokens{}, err
	}
	valid, err := u.passwords.Verify(ctx, model.Text(r, "password_hash"), in.Password)
	if err != nil {
		return auth.Tokens{}, err
	}
	if !valid {
		return auth.Tokens{}, apperror.ErrUnauthorized
	}
	return u.signIn(ctx, model.Text(r, "id"), in.DeviceID, in.UserAgent, in.IP)
}
func (u *UseCase) signIn(ctx context.Context, id, device, agent, ip string) (auth.Tokens, error) {
	var out auth.Tokens
	if !model.UUID(device) || len(agent) > 1000 || len(ip) > 45 {
		return out, apperror.ErrInvalid
	}
	access, ah, err := token()
	if err != nil {
		return out, err
	}
	refresh, rh, err := token()
	if err != nil {
		return out, err
	}
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		r, err := u.repo.LockUser(ctx, id)
		if err != nil {
			return err
		}
		if !model.Flag(r, "is_active") || r["email_verified_at"] == nil {
			return apperror.ErrUnauthorized
		}
		if err = u.repo.ExpireSessions(ctx, id); err != nil {
			return err
		}
		count, err := u.repo.CountSessions(ctx, id, device)
		if err != nil {
			return err
		}
		if model.Number(count, "count") >= 3 {
			return apperror.Public(apperror.ErrConflict, "Three devices are already active")
		}
		_, err = u.repo.SaveSession(ctx, id, device, hex.EncodeToString(rh), ah, agent, ip)
		return err
	})
	if err != nil {
		return out, err
	}
	profile, err := u.repo.Profile(ctx, id)
	if err != nil {
		return out, err
	}
	return auth.Tokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: 900, User: profile}, nil
}
func (u *UseCase) AuthenticateUser(ctx context.Context, raw string) (auth.UserIdentity, error) {
	if len(raw) < 32 || len(raw) > 200 {
		return auth.UserIdentity{}, apperror.ErrUnauthorized
	}
	r, err := u.repo.Authenticate(ctx, digest(raw))
	if errors.Is(err, apperror.ErrNotFound) {
		err = apperror.ErrUnauthorized
	}
	return auth.UserIdentity{UserID: model.Text(r, "user_id")}, err
}
func (u *UseCase) Refresh(ctx context.Context, raw string) (auth.Tokens, error) {
	var out auth.Tokens
	if len(raw) < 32 || len(raw) > 200 {
		return out, apperror.ErrUnauthorized
	}
	old := hex.EncodeToString(digest(raw))
	s, err := u.repo.FindRefresh(ctx, old)
	if errors.Is(err, apperror.ErrNotFound) {
		return out, apperror.ErrUnauthorized
	}
	if err != nil {
		return out, err
	}
	access, ah, err := token()
	if err != nil {
		return out, err
	}
	refresh, rh, err := token()
	if err != nil {
		return out, err
	}
	id := model.Text(s, "user_id")
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		user, err := u.repo.LockUser(ctx, id)
		if err != nil {
			return err
		}
		if !model.Flag(user, "is_active") || user["email_verified_at"] == nil {
			return apperror.ErrUnauthorized
		}
		_, err = u.repo.Rotate(ctx, model.Text(s, "id"), hex.EncodeToString(rh), ah, old)
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.ErrUnauthorized
		}
		return err
	})
	if err != nil {
		return out, err
	}
	profile, err := u.repo.Profile(ctx, id)
	if err != nil {
		return out, err
	}
	return auth.Tokens{AccessToken: access, RefreshToken: refresh, ExpiresIn: 900, User: profile}, nil
}
func (u *UseCase) Logout(ctx context.Context, id, raw string) error {
	return u.repo.Logout(ctx, id, digest(raw))
}
func (u *UseCase) Sessions(ctx context.Context, id string) ([]model.Record, error) {
	return u.repo.Sessions(ctx, id)
}
func (u *UseCase) Revoke(ctx context.Context, id, session string) error {
	if !model.UUID(session) {
		return apperror.ErrInvalid
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := u.repo.LockUser(ctx, id); err != nil {
			return err
		}
		return u.repo.Revoke(ctx, id, session)
	})
}
func (u *UseCase) Profile(ctx context.Context, id string) (model.Record, error) {
	return u.repo.Profile(ctx, id)
}
