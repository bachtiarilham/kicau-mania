// Package usecase owns account authentication and coordinates its transactional state.
package usecase

import (
	"context"
	"time"

	auth "kicau.cloud/control/internal/auth/model"
	"kicau.cloud/control/internal/core/model"
)

type Repository interface {
	ByEmail(ctx context.Context, email string) (model.Record, error)
	ByID(ctx context.Context, id string) (model.Record, error)
	LockUser(ctx context.Context, id string) (model.Record, error)
	CreateUser(ctx context.Context, name, email, phone string, password any, verified bool) (model.Record, error)
	VerifyEmail(ctx context.Context, id string) error
	Password(ctx context.Context, id, hash string) error
	RetireToken(ctx context.Context, id, purpose string) error
	CreateToken(ctx context.Context, id, purpose string, hash []byte, expiry string) error
	FindToken(ctx context.Context, hash []byte, purpose string) (model.Record, error)
	ConsumeToken(ctx context.Context, id string) (model.Record, error)
	ExpireSessions(ctx context.Context, id string) error
	CountSessions(ctx context.Context, id, device string) (model.Record, error)
	SaveSession(ctx context.Context, id, device, refresh string, access []byte, agent, ip string) (model.Record, error)
	Authenticate(ctx context.Context, hash []byte) (model.Record, error)
	FindRefresh(ctx context.Context, hash string) (model.Record, error)
	Rotate(ctx context.Context, id, refresh string, access []byte, old string) (model.Record, error)
	Sessions(ctx context.Context, id string) ([]model.Record, error)
	Revoke(ctx context.Context, id, session string) error
	Logout(ctx context.Context, id string, hash []byte) error
	Provider(ctx context.Context, subject string) (model.Record, error)
	Link(ctx context.Context, id, subject string) error
	Profile(ctx context.Context, id string) (model.Record, error)
	Grant(ctx context.Context, id, role string) error
}

type Transactions interface {
	Do(context.Context, func(context.Context) error) error
}
type Passwords interface {
	Hash(context.Context, string) (string, error)
	Verify(context.Context, string, string) (bool, error)
}
type Mailer interface {
	Ready() bool
	Send(context.Context, string, string, string) error
}
type Google interface {
	Verify(context.Context, string) (auth.GoogleIdentity, error)
}
type Limiter interface {
	Allow(string, int, time.Duration) bool
}
type UseCase struct {
	repo      Repository
	tx        Transactions
	passwords Passwords
	mail      Mailer
	google    Google
	limiter   Limiter
}

func New(r Repository, tx Transactions, p Passwords, m Mailer, g Google, l Limiter) *UseCase {
	return &UseCase{r, tx, p, m, g, l}
}
