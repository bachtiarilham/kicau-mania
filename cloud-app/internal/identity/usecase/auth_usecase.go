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
	SavePreAuth(context.Context, string, model.SessionToken, time.Duration) error
	ConsumePreAuth(context.Context, string) (model.SessionToken, error)
	DeleteBySessionID(context.Context, string) error
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
	dummyHash string
}

func NewAuthUsecase(
	tx contract.TransactionManager,
	users contract.UserRepository,
	sessions contract.SessionRepository,
	tokens TokensStore,
	passwords PasswordHasher,
	limiter RateLimiter,
) *AuthUsecase {
	dummyHash, _ := passwords.Hash(context.Background(), "dummy-security-password-hash-2026")
	return &AuthUsecase{
		tx:        tx,
		users:     users,
		sessions:  sessions,
		tokens:    tokens,
		passwords: passwords,
		limiter:   limiter,
		dummyHash: dummyHash,
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

func sanitizeHeaderValue(val string, maxLen int) string {
	val = strings.TrimSpace(val)
	if len(val) > maxLen {
		val = val[:maxLen]
	}
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, val)
}

func buildAccessibleScopes(profile *model.UserProfile) []model.AccessibleScope {
	scopes := make([]model.AccessibleScope, 0, len(profile.Roles)+len(profile.ArenaRoles))
	for _, r := range profile.Roles {
		scopes = append(scopes, model.AccessibleScope{
			ScopeID:   "global:" + r,
			Role:      r,
			ScopeType: "GLOBAL",
		})
	}
	for _, ar := range profile.ArenaRoles {
		arenaID := ar.ArenaID
		scopes = append(scopes, model.AccessibleScope{
			ScopeID:   "arena:" + arenaID + ":" + ar.Role,
			Role:      ar.Role,
			ScopeType: "ARENA",
			ArenaID:   &arenaID,
		})
	}
	return scopes
}

func findScope(scopes []model.AccessibleScope, scopeID string) (model.AccessibleScope, bool) {
	for _, s := range scopes {
		if s.ScopeID == scopeID {
			return s, true
		}
	}
	return model.AccessibleScope{}, false
}

func (u *AuthUsecase) Login(ctx context.Context, in model.LoginRequest) (*model.PreAuthResponse, error) {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.DeviceID = sanitizeHeaderValue(in.DeviceID, 128)
	in.UserAgent = sanitizeHeaderValue(in.UserAgent, 1024)
	if !u.limiter.Allow("login:"+in.IP, 30, 15*time.Minute) || !u.limiter.Allow("email:"+in.Email, 10, 15*time.Minute) {
		return nil, apperror.ErrRateLimited
	}
	if address, err := mail.ParseAddress(in.Email); err != nil || address.Address != in.Email || len(in.Email) > 254 || len(in.Password) > 256 || len(in.Password) < 12 || len(in.DeviceID) < 8 {
		return nil, apperror.ErrInvalid
	}
	if err := u.tokens.Ping(ctx); err != nil {
		return nil, err
	}

	user, err := u.users.GetByEmail(ctx, in.Email)
	if err != nil && !errors.Is(err, apperror.ErrNotFound) {
		return nil, err
	}

	hashToCompare := u.dummyHash
	if err == nil && user != nil {
		storedHash, hashErr := u.users.GetPasswordHash(ctx, user.ID)
		if hashErr == nil && storedHash != "" {
			hashToCompare = storedHash
		}
	}

	ok, verifyErr := u.passwords.Verify(ctx, hashToCompare, in.Password)
	if verifyErr != nil {
		return nil, verifyErr
	}
	if err != nil || user == nil || !ok || !user.IsActive || user.EmailVerifiedAt == nil {
		return nil, apperror.ErrUnauthorized
	}

	profile, err := u.users.GetProfile(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	preAuthToken, err := randomSecret()
	if err != nil {
		return nil, err
	}

	preAuthTTL := 5 * time.Minute
	err = u.tokens.SavePreAuth(ctx, digest(preAuthToken), model.SessionToken{
		UserID:    user.ID,
		TokenType: "PRE_AUTH",
	}, preAuthTTL)
	if err != nil {
		return nil, err
	}

	return &model.PreAuthResponse{
		PreAuthToken:     preAuthToken,
		TokenType:        "PRE_AUTH",
		ExpiresIn:        int(preAuthTTL.Seconds()),
		User:             profile,
		AccessibleScopes: buildAccessibleScopes(profile),
	}, nil
}

func (u *AuthUsecase) SelectScope(ctx context.Context, in model.SelectScopeRequest) (*model.AuthTokens, error) {
	in.DeviceID = sanitizeHeaderValue(in.DeviceID, 128)
	in.UserAgent = sanitizeHeaderValue(in.UserAgent, 1024)
	in.ScopeID = strings.TrimSpace(in.ScopeID)
	if strings.TrimSpace(in.PreAuthToken) == "" || in.ScopeID == "" || len(in.DeviceID) < 8 {
		return nil, apperror.ErrInvalid
	}

	preAuthSession, err := u.tokens.ConsumePreAuth(ctx, digest(in.PreAuthToken))
	if err != nil {
		return nil, err
	}
	if preAuthSession.TokenType != "PRE_AUTH" || preAuthSession.UserID == "" {
		return nil, apperror.ErrUnauthorized
	}

	var out *model.AuthTokens
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		lockedUser, err := u.users.GetByID(txCtx, preAuthSession.UserID)
		if err != nil {
			return err
		}
		if !lockedUser.IsActive {
			return apperror.ErrUnauthorized
		}

		profile, err := u.users.GetProfile(txCtx, preAuthSession.UserID)
		if err != nil {
			return err
		}

		chosenScope, ok := findScope(buildAccessibleScopes(profile), in.ScopeID)
		if !ok {
			return apperror.ErrForbidden
		}

		if err := u.sessions.ExpireOldSessions(txCtx, preAuthSession.UserID); err != nil {
			return err
		}

		sessionID, expiresAt, err := u.sessions.SaveSession(txCtx, preAuthSession.UserID, in.DeviceID, in.UserAgent, time.Now().Add(30*24*time.Hour))
		if err != nil {
			return err
		}

		out, err = u.issueTokens(txCtx, model.SessionToken{
			UserID:        preAuthSession.UserID,
			SessionID:     sessionID,
			TokenType:     "SCOPED_ACCESS",
			ActiveRole:    chosenScope.Role,
			ActiveArenaID: chosenScope.ArenaID,
		}, "", expiresAt, &chosenScope)
		return err
	})

	return out, err
}

func (u *AuthUsecase) SwitchScope(ctx context.Context, actorID, bearerToken string, in model.SwitchScopeRequest) (*model.AuthTokens, error) {
	in.ScopeID = strings.TrimSpace(in.ScopeID)
	if in.ScopeID == "" || bearerToken == "" {
		return nil, apperror.ErrInvalid
	}

	session, err := u.tokens.Resolve(ctx, "access", digest(bearerToken))
	if err != nil {
		return nil, err
	}
	if session.UserID != actorID || session.TokenType == "PRE_AUTH" {
		return nil, apperror.ErrUnauthorized
	}

	var out *model.AuthTokens
	err = u.tx.Do(ctx, func(txCtx context.Context) error {
		profile, err := u.users.GetProfile(txCtx, actorID)
		if err != nil {
			return err
		}
		chosenScope, ok := findScope(buildAccessibleScopes(profile), in.ScopeID)
		if !ok {
			return apperror.ErrForbidden
		}

		sessions, err := u.sessions.ListSessions(txCtx, actorID)
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

		session.TokenType = "SCOPED_ACCESS"
		session.ActiveRole = chosenScope.Role
		session.ActiveArenaID = chosenScope.ArenaID
		out, err = u.issueTokens(txCtx, session, "", currentSession.ExpiresAt, &chosenScope)
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
	if session.TokenType == "PRE_AUTH" {
		return nil, apperror.ErrUnauthorized
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

		var activeScope *model.AccessibleScope
		if session.ActiveRole != "" {
			scopeType := "GLOBAL"
			scopeID := "global:" + session.ActiveRole
			if session.ActiveArenaID != nil {
				scopeType = "ARENA"
				scopeID = "arena:" + *session.ActiveArenaID + ":" + session.ActiveRole
			}
			activeScope = &model.AccessibleScope{
				ScopeID:   scopeID,
				Role:      session.ActiveRole,
				ScopeType: scopeType,
				ArenaID:   session.ActiveArenaID,
			}
		}

		out, err = u.issueTokens(txCtx, session, refreshToken, currentSession.ExpiresAt, activeScope)
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
	if session.TokenType == "PRE_AUTH" {
		return empty, apperror.ErrUnauthorized
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
	if err := u.sessions.RevokeSession(ctx, actorID, sessionID); err != nil {
		return err
	}
	return u.tokens.DeleteBySessionID(ctx, sessionID)
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

func (u *AuthUsecase) issueTokens(ctx context.Context, s model.SessionToken, oldRefresh string, expiresAt time.Time, activeScope *model.AccessibleScope) (*model.AuthTokens, error) {
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

	s.TokenType = "SCOPED_ACCESS"
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
		ActiveScope:  activeScope,
		User:         profile,
	}, nil
}
