package repository

import (
	"context"
	"time"

	"kicau.cloud/control/internal/identity/contract"
	"kicau.cloud/control/internal/identity/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type sessionRepositoryImpl struct {
	store *postgres.Store
}

func NewSessionRepository(s *postgres.Store) contract.SessionRepository {
	return &sessionRepositoryImpl{store: s}
}

func (r *sessionRepositoryImpl) ListSessions(ctx context.Context, userID string) ([]model.SessionView, error) {
	query := `
		SELECT id, user_id, device_id, expires_at, revoked_at, user_agent
		FROM cloud_read.v_user_sessions
		WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP
		ORDER BY expires_at DESC
	`
	return postgres.QueryMany[model.SessionView](ctx, r.store, query, userID)
}

func (r *sessionRepositoryImpl) RevokeSession(ctx context.Context, userID, sessionID string) error {
	query := `UPDATE iam.user_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE id = $2::uuid AND user_id = $1::uuid AND revoked_at IS NULL`
	return r.store.ExecSQL(ctx, query, userID, sessionID)
}

func (r *sessionRepositoryImpl) SaveSession(ctx context.Context, userID, deviceID, userAgent string, expiresAt time.Time) (string, time.Time, error) {
	query := `
		INSERT INTO iam.user_sessions(user_id, device_id, expires_at, user_agent)
		VALUES($1::uuid, $2, $3, $4)
		RETURNING id, expires_at
	`
	row, err := r.store.QueryRow(ctx, query, userID, deviceID, expiresAt, userAgent)
	if err != nil {
		return "", time.Time{}, err
	}
	var id string
	var exp time.Time
	if err := row.Scan(&id, &exp); err != nil {
		return "", time.Time{}, err
	}
	return id, exp, nil
}

func (r *sessionRepositoryImpl) RevokeAllSessions(ctx context.Context, userID string) error {
	query := `UPDATE iam.user_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = $1::uuid AND revoked_at IS NULL`
	return r.store.ExecSQL(ctx, query, userID)
}

func (r *sessionRepositoryImpl) ExpireOldSessions(ctx context.Context, userID string) error {
	query := `UPDATE iam.user_sessions SET revoked_at = CURRENT_TIMESTAMP WHERE user_id = $1::uuid AND revoked_at IS NULL AND expires_at <= CURRENT_TIMESTAMP`
	return r.store.ExecSQL(ctx, query, userID)
}

func (r *sessionRepositoryImpl) OtherDeviceCount(ctx context.Context, userID, currentDeviceID string) (int, error) {
	query := `SELECT count(*) FROM iam.user_sessions WHERE user_id = $1::uuid AND device_id <> $2 AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP`
	row, err := r.store.QueryRow(ctx, query, userID, currentDeviceID)
	if err != nil {
		return 0, err
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
