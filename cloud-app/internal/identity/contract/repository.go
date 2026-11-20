package contract

import (
	"context"
	"time"

	"kicau.cloud/control/internal/identity/model"
)

type TransactionManager interface {
	Do(ctx context.Context, f func(context.Context) error) error
}

type UserRepository interface {
	GetByID(ctx context.Context, id string) (*model.UserView, error)
	GetByEmail(ctx context.Context, email string) (*model.UserView, error)
	GetProfile(ctx context.Context, id string) (*model.UserProfile, error)
	List(ctx context.Context, filter model.UserFilter) ([]model.UserView, error)
	Create(ctx context.Context, id string, req *model.CreateUserRequest, passwordHash *string) (*model.UserView, error)
	Update(ctx context.Context, id string, req *model.UpdateUserRequest) (*model.UserView, error)
	Delete(ctx context.Context, id string, expectedVersion int) error
	GetPasswordHash(ctx context.Context, id string) (string, error)
	UpdatePassword(ctx context.Context, id string, newHash string) error
	SetMutationContext(ctx context.Context, actorID string, arenaID *string) error
	BootstrapLock(ctx context.Context) error
	AdminExists(ctx context.Context) (bool, error)
	VerifyEmail(ctx context.Context, id string) error
	AssignGlobalRole(ctx context.Context, id, role string) error
}

type SessionRepository interface {
	ListSessions(ctx context.Context, userID string) ([]model.SessionView, error)
	RevokeSession(ctx context.Context, userID, sessionID string) error
	SaveSession(ctx context.Context, userID, deviceID, userAgent string, expiresAt time.Time) (string, time.Time, error)
	RevokeAllSessions(ctx context.Context, userID string) error
	ExpireOldSessions(ctx context.Context, userID string) error
	OtherDeviceCount(ctx context.Context, userID, currentDeviceID string) (int, error)
}
