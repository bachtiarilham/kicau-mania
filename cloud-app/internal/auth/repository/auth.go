// Package repository persists authentication state and sessions without calling other repositories.
package repository

import (
	"context"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type Repository struct{ db *postgres.Store }

func New(db *postgres.Store) *Repository { return &Repository{db} }
func (r *Repository) ByEmail(ctx context.Context, email string) (model.Record, error) {
	return r.db.One(ctx, "AuthByEmail", email)
}
func (r *Repository) ByID(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "AuthByID", id)
}
func (r *Repository) LockUser(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockUser", id)
}
func (r *Repository) CreateUser(ctx context.Context, name, email, phone string, password any, verified bool) (model.Record, error) {
	return r.db.One(ctx, "CreateUser", name, email, phone, password, verified)
}
func (r *Repository) VerifyEmail(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "VerifyEmail", id)
}
func (r *Repository) Password(ctx context.Context, id, hash string) error {
	return r.db.Exec(ctx, "PasswordUpdate", id, hash)
}
func (r *Repository) RetireToken(ctx context.Context, id, purpose string) error {
	return r.db.Exec(ctx, "TokenRetire", id, purpose)
}
func (r *Repository) CreateToken(ctx context.Context, id, purpose string, hash []byte, expiry string) error {
	return r.db.Exec(ctx, "TokenCreate", id, purpose, hash, expiry)
}
func (r *Repository) FindToken(ctx context.Context, hash []byte, purpose string) (model.Record, error) {
	return r.db.One(ctx, "TokenFind", hash, purpose)
}
func (r *Repository) ConsumeToken(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "TokenConsume", id)
}
func (r *Repository) ExpireSessions(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "SessionExpire", id)
}
func (r *Repository) CountSessions(ctx context.Context, id, device string) (model.Record, error) {
	return r.db.One(ctx, "SessionCount", id, device)
}
func (r *Repository) SaveSession(ctx context.Context, id, device, refresh string, access []byte, agent, ip string) (model.Record, error) {
	return r.db.One(ctx, "SessionSave", id, device, refresh, access, agent, ip)
}
func (r *Repository) Authenticate(ctx context.Context, hash []byte) (model.Record, error) {
	return r.db.One(ctx, "SessionAuthenticate", hash)
}
func (r *Repository) FindRefresh(ctx context.Context, hash string) (model.Record, error) {
	return r.db.One(ctx, "RefreshFind", hash)
}
func (r *Repository) Rotate(ctx context.Context, id, refresh string, access []byte, old string) (model.Record, error) {
	return r.db.One(ctx, "RefreshRotate", id, refresh, access, old)
}
func (r *Repository) Sessions(ctx context.Context, id string) ([]model.Record, error) {
	return r.db.Many(ctx, "SessionList", id)
}
func (r *Repository) Revoke(ctx context.Context, id, session string) error {
	return r.db.Exec(ctx, "SessionRevoke", id, session)
}
func (r *Repository) Logout(ctx context.Context, id string, hash []byte) error {
	return r.db.Exec(ctx, "SessionLogout", id, hash)
}
func (r *Repository) Provider(ctx context.Context, subject string) (model.Record, error) {
	return r.db.One(ctx, "ProviderFind", subject)
}
func (r *Repository) Link(ctx context.Context, id, subject string) error {
	return r.db.Exec(ctx, "ProviderLink", id, subject)
}
func (r *Repository) Profile(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "UserProfile", id)
}
func (r *Repository) Grant(ctx context.Context, id, role string) error {
	return r.db.Exec(ctx, "AssignGlobalRole", id, role)
}
