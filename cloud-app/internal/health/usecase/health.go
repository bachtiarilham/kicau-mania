// Package usecase determines operational readiness through a database health port.
package usecase

import (
	"context"
	"time"

	"kicau.cloud/control/internal/apperror"
)

type Database interface{ Ping(context.Context) error }

type UseCase struct {
	database Database
	timeout  time.Duration
}

func New(database Database, timeout time.Duration) *UseCase {
	return &UseCase{database: database, timeout: timeout}
}

func (u *UseCase) Ready(ctx context.Context) error {
	if u.database == nil {
		return apperror.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()
	if err := u.database.Ping(ctx); err != nil {
		return apperror.ErrUnavailable
	}
	return nil
}
