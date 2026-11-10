package repository

import (
	"context"

	"kicau.cloud/control/internal/core/model"
)

func (r *Repository) Arena(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "EntityArenasGet", id)
}
func (r *Repository) LockArena(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockArena", id)
}
func (r *Repository) LockRound(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "LockRound", id)
}
func (r *Repository) Criteria(ctx context.Context, arena string) ([]model.Record, error) {
	return r.db.Many(ctx, "ArenaCriteria", arena)
}
func (r *Repository) Items(ctx context.Context, round string) ([]model.Record, error) {
	return r.db.Many(ctx, "RoundItems", round)
}
func (r *Repository) AllItems(ctx context.Context, round string) ([]model.Record, error) {
	return r.db.Many(ctx, "RoundAllItems", round)
}
func (r *Repository) Judges(ctx context.Context, round string) ([]model.Record, error) {
	return r.db.Many(ctx, "RoundJudges", round)
}
func (r *Repository) Recap(ctx context.Context, round string) ([]model.Record, error) {
	return r.db.Many(ctx, "LiveRecap", round)
}
func (r *Repository) LockCriteria(ctx context.Context, arena string) error {
	return r.db.Exec(ctx, "LockCriteria", arena)
}
func (r *Repository) Phase(ctx context.Context, id, phase string, from, winner, actor any) (model.Record, error) {
	return r.db.One(ctx, "SetRoundPhase", id, phase, from, winner, actor)
}
func (r *Repository) AssignJudge(ctx context.Context, arena, round, user string) (model.Record, error) {
	return r.db.One(ctx, "AssignJudge", arena, round, user)
}
func (r *Repository) RemoveJudge(ctx context.Context, id, round string) (model.Record, error) {
	return r.db.One(ctx, "RemoveJudge", id, round)
}
func (r *Repository) Registrant(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "UserProfile", id)
}
func (r *Repository) Bird(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "EntityBirdsGet", id)
}
func (r *Repository) CreateHeader(ctx context.Context, arena string, user any, actor, source, name, phone string) (model.Record, error) {
	return r.db.One(ctx, "CreateRegistration", arena, user, actor, source, name, phone)
}
func (r *Repository) InsertItem(ctx context.Context, arena, reg, round, bird string, fee any) error {
	return r.db.Exec(ctx, "InsertRegistrationItem", arena, reg, round, bird, fee)
}
func (r *Repository) SetTotal(ctx context.Context, id string, total any, name, phone string) error {
	return r.db.Exec(ctx, "SetRegistrationTotal", id, total, name, phone)
}
func (r *Repository) Header(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "RegistrationGet", id)
}
func (r *Repository) LockHeader(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "RegistrationLock", id)
}
func (r *Repository) Registrations(ctx context.Context, cursor, arena, actor string, staff bool, limit int) ([]model.Record, error) {
	return r.db.Many(ctx, "RegistrationList", cursor, arena, actor, staff, limit)
}
func (r *Repository) HeaderItems(ctx context.Context, id string) ([]model.Record, error) {
	return r.db.Many(ctx, "RegistrationItems", id)
}
func (r *Repository) Payments(ctx context.Context, id string) ([]model.Record, error) {
	return r.db.Many(ctx, "RegistrationPayments", id)
}
func (r *Repository) ClearItems(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "DeleteRegistrationItems", id)
}
func (r *Repository) CancelHeader(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "CancelRegistration", id)
}
func (r *Repository) CancelItems(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "CancelRegistrationItems", id)
}
func (r *Repository) CancelPayments(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "CancelPendingPayments", id)
}
func (r *Repository) CreatePayment(ctx context.Context, arena, reg string, amount any, method, proof string) (model.Record, error) {
	return r.db.One(ctx, "PaymentCreate", arena, reg, amount, method, proof)
}
func (r *Repository) Payment(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "PaymentGet", id)
}
func (r *Repository) LockPayment(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "PaymentLock", id)
}
func (r *Repository) VerifyPayment(ctx context.Context, id, actor string) (model.Record, error) {
	return r.db.One(ctx, "PaymentVerify", id, actor)
}
func (r *Repository) RejectPayment(ctx context.Context, id string) (model.Record, error) {
	return r.db.One(ctx, "PaymentReject", id)
}
func (r *Repository) ConfirmItem(ctx context.Context, id, reg, gantangan string) (model.Record, error) {
	return r.db.One(ctx, "ConfirmItem", id, reg, gantangan)
}
func (r *Repository) ConfirmHeader(ctx context.Context, id string) error {
	return r.db.Exec(ctx, "ConfirmRegistration", id)
}

func (r *Repository) Recordings(ctx context.Context, id string) ([]model.Record, error) {
	return r.db.Many(ctx, "RegistrationRecordings", id)
}
