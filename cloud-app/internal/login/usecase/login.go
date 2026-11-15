package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"kicau.cloud/control/internal/apperror"
	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
	input "kicau.cloud/control/internal/login/model"
	"net/mail"
	"strconv"
	"strings"
	"time"
)

type Data interface {
	One(context.Context, string, ...any) (model.Record, error)
	Exec(context.Context, string, ...any) error
	Do(context.Context, func(context.Context) error) error
}
type Tokens interface {
	Resolve(context.Context, string, string) (input.Session, error)
	Rotate(context.Context, string, input.Session, time.Duration, time.Duration) error
	Delete(context.Context, input.Session) error
	Ping(context.Context) error
}
type Passwords interface {
	Verify(context.Context, string, string) (bool, error)
}
type Limiter interface {
	Allow(string, int, time.Duration) bool
}
type UseCase struct {
	db        Data
	tokens    Tokens
	passwords Passwords
	limiter   Limiter
}

func New(d Data, t Tokens, p Passwords, l Limiter) *UseCase { return &UseCase{d, t, p, l} }
func digest(v string) string                                { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func secret() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func (u *UseCase) Login(c context.Context, in auth.Login) (out auth.Tokens, e error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !u.limiter.Allow("login:"+in.IP, 30, 15*time.Minute) || !u.limiter.Allow("email:"+strings.ToLower(in.Email), 10, 15*time.Minute) {
		return out, apperror.ErrRateLimited
	}
	if address, e := mail.ParseAddress(in.Email); e != nil || address.Address != in.Email || len(in.Email) > 254 || len(in.Password) > 256 || len(in.Password) < 12 || len(in.DeviceID) < 8 || len(in.DeviceID) > 128 || len(in.UserAgent) > 1024 {
		return out, apperror.ErrInvalid
	}
	if e = u.tokens.Ping(c); e != nil {
		return
	}
	account, e := u.db.One(c, "LoginAccountByEmail", in.Email)
	if errors.Is(e, apperror.ErrNotFound) {
		return out, apperror.ErrUnauthorized
	}
	if e != nil {
		return out, e
	}
	ok, e := u.passwords.Verify(c, model.Text(account, "password_hash"), in.Password)
	if e != nil {
		return out, e
	}
	if !ok || !model.Flag(account, "is_active") || account["email_verified_at"] == nil {
		return out, apperror.ErrUnauthorized
	}
	user := model.Text(account, "id")
	e = u.db.Do(c, func(c context.Context) error {
		a, e := u.db.One(c, "LoginUserLock", user)
		if e != nil {
			return e
		}
		if !model.Flag(a, "is_active") {
			return apperror.ErrUnauthorized
		}
		current, e := u.db.One(c, "AccountCredentialByID", user)
		if e != nil {
			return e
		}
		if model.Text(current, "password_hash") != model.Text(account, "password_hash") {
			return apperror.ErrUnauthorized
		}
		if e = u.db.Exec(c, "LoginExpireSessions", user); e != nil {
			return e
		}
		n, e := u.db.One(c, "LoginOtherDeviceCount", user, in.DeviceID)
		if e != nil {
			return e
		}
		count, _ := strconv.Atoi(fmt.Sprint(n["count"]))
		if count >= 3 {
			return apperror.Public(apperror.ErrConflict, "Maksimal tiga perangkat aktif; cabut sesi terlebih dahulu")
		}
		s, e := u.db.One(c, "LoginSessionSave", user, in.DeviceID, time.Now().Add(30*24*time.Hour), in.UserAgent)
		if e != nil {
			return e
		}
		out, e = u.issue(c, input.Session{UserID: user, SessionID: model.Text(s, "id")}, "", model.Text(s, "expires_at"))
		return e
	})
	return
}
func (u *UseCase) issue(c context.Context, s input.Session, old, expiry string) (out auth.Tokens, e error) {
	expires, e := time.Parse(time.RFC3339Nano, expiry)
	if e != nil {
		return out, apperror.ErrUnavailable
	}
	ttl := time.Until(expires)
	if ttl < time.Second {
		return out, apperror.ErrUnauthorized
	}
	aTTL := 15 * time.Minute
	if ttl < aTTL {
		aTTL = ttl
	}
	out.AccessToken, e = secret()
	if e != nil {
		return
	}
	out.RefreshToken, e = secret()
	if e != nil {
		return
	}
	s.Access = digest(out.AccessToken)
	s.Refresh = digest(out.RefreshToken)
	out.ExpiresIn = int(aTTL.Seconds())
	out.User, e = u.db.One(c, "ActorProfile", s.UserID)
	if e != nil {
		return
	}
	e = u.tokens.Rotate(c, old, s, aTTL, ttl)
	return
}
func (u *UseCase) validate(c context.Context, kind, token string) (input.Session, model.Record, error) {
	if len(token) != 43 {
		return input.Session{}, nil, apperror.ErrUnauthorized
	}
	s, e := u.tokens.Resolve(c, kind, digest(token))
	if e != nil {
		return s, nil, e
	}
	r, e := u.db.One(c, "LoginSessionValidate", s.SessionID, s.UserID)
	if errors.Is(e, apperror.ErrNotFound) {
		e = apperror.ErrUnauthorized
	}
	return s, r, e
}
func (u *UseCase) AuthenticateUser(c context.Context, token string) (auth.UserIdentity, error) {
	s, _, e := u.validate(c, "a", token)
	return auth.UserIdentity{UserID: s.UserID}, e
}
func (u *UseCase) Refresh(c context.Context, token string) (auth.Tokens, error) {
	s, r, e := u.validate(c, "r", token)
	if e != nil {
		return auth.Tokens{}, e
	}
	var out auth.Tokens
	e = u.db.Do(c, func(c context.Context) error {
		if _, e := u.db.One(c, "LoginUserLock", s.UserID); e != nil {
			return e
		}
		if _, e := u.db.One(c, "LoginSessionValidate", s.SessionID, s.UserID); e != nil {
			return apperror.ErrUnauthorized
		}
		var e error
		out, e = u.issue(c, s, digest(token), model.Text(r, "expires_at"))
		return e
	})
	return out, e
}
func (u *UseCase) Logout(c context.Context, token string) error {
	s, _, e := u.validate(c, "r", token)
	if e != nil {
		return e
	}
	if e = u.db.Exec(c, "LoginSessionRevoke", s.UserID, s.SessionID); e != nil {
		return e
	}
	return u.tokens.Delete(c, s)
}
