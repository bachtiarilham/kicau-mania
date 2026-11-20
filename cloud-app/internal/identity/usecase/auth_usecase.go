package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"kicau.cloud/control/internal/apperror"
	authModel "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/identity/contract"
	"kicau.cloud/control/internal/identity/model"
)

type TokensStore interface {
	Resolve(context.Context, string, string) (model.SessionToken, error)
	Rotate(context.Context, string, model.SessionToken, time.Duration, time.Duration) error
	Delete(context.Context, model.SessionToken) error
	Ping(context.Context) error
}

type PasswordHasher interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}

type RateLimiter interface {
	Allow(string, int, time.Duration) bool
}

type AuthUsecase struct {
	tx        contract.TransactionManager
	users     contract.UserRepository
	sessions  contract.SessionRepository
	tokens    TokensStore
	passwords PasswordHasher
	limiter   RateLimiter
}

func NewAuthUsecase(
	tx contract.TransactionManager,
	users contract.UserRepository,
	sessions contract.SessionRepository,
	tokens TokensStore,
	passwords PasswordHasher,
	limiter RateLimiter,
) *AuthUsecase {
	return &AuthUsecase{
		tx:        tx,
		users:     users,
		sessions:  sessions,
		tokens:    tokens,
		passwords: passwords,
		limiter:   limiter,
	}
}

func digest(v string) string {
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:])
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (u *AuthUsecase) Login(ctx context.Context, in model.LoginRequest) (*model.AuthTokens, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !u.limiter.Allow("login:"+in.IP, 30, 15*time.Minute) || !u.limiter.Allow("email:"+strings.ToLower(in.Email), 10, 15*time.Minute) {
		return nil, apperror.ErrRateLimited
	}
	if address, err := mail.ParseAddress(in.Email); err != nil || address.Address != in.Email || len(in.Email) > 254 || len(in.Password) > 256 || len(in.Password) < 12 || len(in.DeviceID) < 8 || len(in.DeviceID) > 128 || len(in.UserAgent) > 1024 {
		return nil, apperror.ErrInvalid
	}
	if err := u.tokens.Ping(ctx); err != nil {
		return nil, err
	}

	user, err := u.users.GetByEmail(ctx, in.Email)
	if errors.Is(err, apperror.ErrNotFound) {
		return nil, apperror.ErrUnauthorized
	}
	if err != nil {
		return nil, err
	}

	hash, err := u.users.GetPasswordHash(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	ok, err := u.passwords.Verify(ctx, hash, in.Password)
	if err != nil {
		return nil, err
	}
	if !ok || !user.IsActive || user.EmailVerifiedAt == nil {
		return nil, apperror.ErrUnauthorized
	}

	var out *model.AuthTokens
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		lockedUser, err := u.users.GetByID(txCtx, user.ID)
		if err != nil {
			return err
		}
		if !lockedUser.IsActive {
			return apperror.ErrUnauthorized
		}

		if err := u.sessions.ExpireOldSessions(txCtx, user.ID); err != nil {
			return err
		}

		count, err := u.sessions.OtherDeviceCount(txCtx, user.ID, in.DeviceID)
		if err != nil {
			return err
		}
		if count >= 3 {
			return apperror.Public(apperror.ErrConflict, "Maksimal tiga perangkat aktif; cabut sesi terlebih dahulu")
		}

		sessionID, expiresAt, err := u.sessions.SaveSession(txCtx, user.ID, in.DeviceID, in.UserAgent, time.Now().Add(30*24*time.Hour))
		if err != nil {
			return err
		}

		out, err = u.issueTokens(txCtx, model.SessionToken{UserID: user.ID, SessionID: sessionID}, "", expiresAt)
		return err
	})

	return out, err
}

func (u *AuthUsecase) Refresh(ctx context.Context, refreshToken string) (*model.AuthTokens, error) {
	if refreshToken == "" {
		return nil, apperror.ErrUnauthorized
	}
	session, err := u.tokens.Resolve(ctx, "refresh", digest(refreshToken))
	if err != nil {
		return nil, err
	}

	var out *model.AuthTokens
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		user, err := u.users.GetByID(txCtx, session.UserID)
		if err != nil {
			return err
		}
		if !user.IsActive {
			return apperror.ErrUnauthorized
		}

		sessions, err := u.sessions.ListSessions(txCtx, session.UserID)
		if err != nil {
			return err
		}
		var currentSession *model.SessionView
		for _, s := range sessions {
			if s.ID == session.SessionID {
				currentSession = &s
				break
			}
		}
		if currentSession == nil {
			return apperror.ErrUnauthorized
		}

		out, err = u.issueTokens(txCtx, session, refreshToken, currentSession.ExpiresAt)
		return err
	})

	return out, err
}

func (u *AuthUsecase) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	session, err := u.tokens.Resolve(ctx, "refresh", digest(refreshToken))
	if err != nil {
		return nil
	}
	_ = u.tokens.Delete(ctx, session)
	return u.sessions.RevokeSession(ctx, session.UserID, session.SessionID)
}

func (u *AuthUsecase) AuthenticateUser(ctx context.Context, bearerToken string) (authModel.UserIdentity, error) {
	var empty authModel.UserIdentity
	if bearerToken == "" {
		return empty, apperror.ErrUnauthorized
	}
	session, err := u.tokens.Resolve(ctx, "access", digest(bearerToken))
	if err != nil {
		return empty, err
	}

	user, err := u.users.GetByID(ctx, session.UserID)
	if err != nil {
		return empty, err
	}
	if !user.IsActive {
		return empty, apperror.ErrUnauthorized
	}

	return authModel.UserIdentity{
		UserID: user.ID,
	}, nil
}

func (u *AuthUsecase) Profile(ctx context.Context, actorID string) (*model.UserProfile, error) {
	return u.users.GetProfile(ctx, actorID)
}

func (u *AuthUsecase) Sessions(ctx context.Context, actorID string) ([]model.SessionView, error) {
	return u.sessions.ListSessions(ctx, actorID)
}

func (u *AuthUsecase) RevokeSession(ctx context.Context, actorID, sessionID string) error {
	return u.sessions.RevokeSession(ctx, actorID, sessionID)
}

func (u *AuthUsecase) ChangePassword(ctx context.Context, actorID, oldPassword, newPassword string) error {
	hash, err := u.users.GetPasswordHash(ctx, actorID)
	if err != nil {
		return err
	}
	ok, err := u.passwords.Verify(ctx, hash, oldPassword)
	if err != nil {
		return err
	}
	if !ok {
		return apperror.Invalid("Password lama salah")
	}

	newHash, err := u.passwords.Hash(ctx, newPassword)
	if err != nil {
		return err
	}

	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.users.UpdatePassword(txCtx, actorID, newHash); err != nil {
			return err
		}
		return u.sessions.RevokeAllSessions(txCtx, actorID)
	})
}

func (u *AuthUsecase) issueTokens(ctx context.Context, s model.SessionToken, oldRefresh string, expiresAt time.Time) (*model.AuthTokens, error) {
	ttl := time.Until(expiresAt)
	if ttl < time.Second {
		return nil, apperror.ErrUnauthorized
	}
	accessTTL := 15 * time.Minute
	if ttl < accessTTL {
		accessTTL = ttl
	}

	accessToken, err := randomSecret()
	if err != nil {
		return nil, err
	}
	refreshToken, err := randomSecret()
	if err != nil {
		return nil, err
	}

	s.Access = digest(accessToken)
	s.Refresh = digest(refreshToken)

	oldHash := ""
	if oldRefresh != "" {
		oldHash = digest(oldRefresh)
	}
	if err := u.tokens.Rotate(ctx, oldHash, s, accessTTL, ttl); err != nil {
		return nil, err
	}

	profile, err := u.users.GetProfile(ctx, s.UserID)
	if err != nil {
		return nil, err
	}

	return &model.AuthTokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTTL.Seconds()),
		User:         profile,
	}, nil
}
