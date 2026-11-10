package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"kicau.cloud/control/internal/apperror"
	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/logger"
)

var phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)

func email(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	a, e := mail.ParseAddress(s)
	if e != nil || a.Address != s || len(s) > 100 {
		return "", apperror.Invalid("Invalid email")
	}
	return s, nil
}
func token() (string, []byte, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", nil, e
	}
	s := base64.RawURLEncoding.EncodeToString(b[:])
	h := sha256.Sum256([]byte(s))
	return s, h[:], nil
}
func digest(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }
func (u *UseCase) limit(key string, n int, d time.Duration) error {
	if !u.limiter.Allow(key, n, d) {
		return apperror.ErrRateLimited
	}
	return nil
}
func (u *UseCase) Register(ctx context.Context, in auth.Register) error {
	if !u.mail.Ready() {
		return apperror.ErrUnavailable
	}
	if err := u.limit("register-ip:"+in.IP, 10, time.Hour); err != nil {
		return err
	}
	e, err := email(in.Email)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 100 || !phonePattern.MatchString(in.Phone) {
		return apperror.ErrInvalid
	}
	if err = u.limit("register:"+e, 3, time.Hour); err != nil {
		return err
	}
	hash, err := u.passwords.Hash(ctx, in.Password)
	if err != nil {
		return err
	}
	var id string
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		row, err := u.repo.CreateUser(ctx, in.Name, e, in.Phone, hash, false)
		if err != nil {
			return err
		}
		id = model.Text(row, "id")
		return u.repo.Grant(ctx, id, "peserta")
	})
	if err != nil {
		return err
	}
	return u.issue(ctx, id, "VERIFY_EMAIL")
}
func (u *UseCase) issue(ctx context.Context, id, purpose string) error {
	raw, hash, err := token()
	if err != nil {
		return err
	}
	var address string
	expiry := "24 hours"
	if purpose == "RESET_PASSWORD" {
		expiry = "30 minutes"
	}
	err = u.tx.Do(ctx, func(ctx context.Context) error {
		row, err := u.repo.LockUser(ctx, id)
		if err != nil {
			return err
		}
		if !model.Flag(row, "is_active") {
			return apperror.ErrForbidden
		}
		address = model.Text(row, "email")
		if err = u.repo.RetireToken(ctx, id, purpose); err != nil {
			return err
		}
		return u.repo.CreateToken(ctx, id, purpose, hash, expiry)
	})
	if err != nil {
		return err
	}
	if err = u.mail.Send(ctx, address, purpose, raw); err != nil {
		logger.Warn("Account email delivery failed; resend is available")
	}
	return nil
}
func (u *UseCase) RequestEmail(ctx context.Context, raw, purpose, ip string) error {
	if !u.mail.Ready() {
		return apperror.ErrUnavailable
	}
	if err := u.limit("mail-ip:"+ip, 12, 15*time.Minute); err != nil {
		return err
	}
	e, err := email(raw)
	if err != nil {
		return err
	}
	if err = u.limit("mail:"+purpose+":"+e, 3, 15*time.Minute); err != nil {
		return err
	}
	row, err := u.repo.ByEmail(ctx, e)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !model.Flag(row, "is_active") {
		return nil
	}
	if purpose == "VERIFY_EMAIL" && row["email_verified_at"] != nil {
		return nil
	}
	if purpose == "RESET_PASSWORD" && model.Text(row, "password_hash") == "" {
		return nil
	}
	return u.issue(ctx, model.Text(row, "id"), purpose)
}
func (u *UseCase) Consume(ctx context.Context, raw, password, purpose string) error {
	if len(raw) < 32 || len(raw) > 200 {
		return apperror.ErrInvalid
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		t, err := u.repo.FindToken(ctx, digest(raw), purpose)
		if errors.Is(err, apperror.ErrNotFound) {
			return apperror.Invalid("Token invalid or expired")
		}
		if err != nil {
			return err
		}
		id := model.Text(t, "user_id")
		user, err := u.repo.LockUser(ctx, id)
		if err != nil {
			return err
		}
		if !model.Flag(user, "is_active") {
			return apperror.ErrForbidden
		}
		if _, err = u.repo.ConsumeToken(ctx, model.Text(t, "id")); err != nil {
			if errors.Is(err, apperror.ErrNotFound) {
				return apperror.Invalid("Token invalid or expired")
			}
			return err
		}
		if err = u.repo.VerifyEmail(ctx, id); err != nil {
			return err
		}
		if purpose == "RESET_PASSWORD" {
			hash, err := u.passwords.Hash(ctx, password)
			if err != nil {
				return err
			}
			if err = u.repo.Password(ctx, id, hash); err != nil {
				return err
			}
			return u.repo.Revoke(ctx, id, "")
		}
		return nil
	})
}
func (u *UseCase) ChangePassword(ctx context.Context, id, old, next string) error {
	if err := u.limit("password-change:"+id, 3, 15*time.Minute); err != nil {
		return err
	}
	hash, err := u.passwords.Hash(ctx, next)
	if err != nil {
		return err
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		r, err := u.repo.LockUser(ctx, id)
		if err != nil {
			return err
		}
		valid, err := u.passwords.Verify(ctx, model.Text(r, "password_hash"), old)
		if err != nil {
			return err
		}
		if !valid {
			return apperror.ErrUnauthorized
		}
		if err = u.repo.Password(ctx, id, hash); err != nil {
			return err
		}
		return u.repo.Revoke(ctx, id, "")
	})
}
func (u *UseCase) Bootstrap(ctx context.Context, in auth.Register) error {
	e, err := email(in.Email)
	if err != nil || !phonePattern.MatchString(in.Phone) || strings.TrimSpace(in.Name) == "" {
		return apperror.ErrInvalid
	}
	h, err := u.passwords.Hash(ctx, in.Password)
	if err != nil {
		return err
	}
	return u.tx.Do(ctx, func(ctx context.Context) error {
		r, err := u.repo.CreateUser(ctx, in.Name, e, in.Phone, h, true)
		if err != nil {
			return err
		}
		return u.repo.Grant(ctx, model.Text(r, "id"), "superadmin")
	})
}
