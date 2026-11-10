package usecase

import (
	"context"
	"errors"
	"time"

	"kicau.cloud/control/internal/apperror"
	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
)

func (u *UseCase) GoogleLogin(ctx context.Context, in auth.GoogleLogin) (auth.Tokens, error) {
	var out auth.Tokens
	if err := u.limit("google:"+in.IP, 10, time.Minute); err != nil {
		return out, err
	}
	identity, err := u.google.Verify(ctx, in.Credential)
	if err != nil {
		return out, err
	}
	r, err := u.repo.Provider(ctx, identity.Subject)
	var id string
	if errors.Is(err, apperror.ErrNotFound) {
		if !phonePattern.MatchString(in.Phone) {
			return out, apperror.Invalid("Phone is required for a new Google account")
		}
		if !identity.Authoritative && !u.mail.Ready() {
			return out, apperror.ErrUnavailable
		}
		existing, e := u.repo.ByEmail(ctx, identity.Email)
		if e == nil && existing != nil {
			return out, apperror.Public(apperror.ErrConflict, "Sign in to the existing account before linking Google")
		}
		if e != nil && !errors.Is(e, apperror.ErrNotFound) {
			return out, e
		}
		err = u.tx.Do(ctx, func(ctx context.Context) error {
			r, e := u.repo.CreateUser(ctx, identity.Name, identity.Email, in.Phone, nil, identity.Authoritative)
			if e != nil {
				return e
			}
			id = model.Text(r, "id")
			if e = u.repo.Grant(ctx, id, "peserta"); e != nil {
				return e
			}
			return u.repo.Link(ctx, id, identity.Subject)
		})
		if err != nil {
			return out, err
		}
		if !identity.Authoritative {
			if err = u.issue(ctx, id, "VERIFY_EMAIL"); err != nil {
				return out, err
			}
			return out, apperror.Public(apperror.ErrForbidden, "Verify your email before signing in")
		}
	} else if err != nil {
		return out, err
	} else {
		id = model.Text(r, "user_id")
	}
	return u.signIn(ctx, id, in.DeviceID, in.UserAgent, in.IP)
}
func (u *UseCase) LinkGoogle(ctx context.Context, id, credential string) error {
	identity, err := u.google.Verify(ctx, credential)
	if err != nil {
		return err
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		if _, err := u.repo.LockUser(ctx, id); err != nil {
			return err
		}
		linked, e := u.repo.Provider(ctx, identity.Subject)
		if e == nil {
			if model.Text(linked, "user_id") == id {
				return nil
			}
			return apperror.ErrConflict
		}
		if !errors.Is(e, apperror.ErrNotFound) {
			return e
		}
		return u.repo.Link(ctx, id, identity.Subject)
	})
}
