// Package usecase coordinates registrations, payment confirmation and round lifecycles.
package usecase

import (
	"context"

	"kicau.cloud/control/internal/core/model"
)

type WorkflowRepository interface {
	Recordings(context.Context, string) ([]model.Record, error)
	Arena(ctx context.Context, id string) (model.Record, error)
	LockArena(ctx context.Context, id string) (model.Record, error)
	LockRound(ctx context.Context, id string) (model.Record, error)
	Criteria(ctx context.Context, arena string) ([]model.Record, error)
	Items(ctx context.Context, round string) ([]model.Record, error)
	AllItems(ctx context.Context, round string) ([]model.Record, error)
	Judges(ctx context.Context, round string) ([]model.Record, error)
	Recap(ctx context.Context, round string) ([]model.Record, error)
	LockCriteria(ctx context.Context, arena string) error
	Phase(ctx context.Context, id, phase string, from, winner, actor any) (model.Record, error)
	AssignJudge(ctx context.Context, arena, round, user string) (model.Record, error)
	RemoveJudge(ctx context.Context, id, round string) (model.Record, error)
	Registrant(ctx context.Context, id string) (model.Record, error)
	Bird(ctx context.Context, id string) (model.Record, error)
	CreateHeader(ctx context.Context, arena string, user any, actor, source, name, phone string) (model.Record, error)
	InsertItem(ctx context.Context, arena, reg, round, bird string, fee any) error
	SetTotal(ctx context.Context, id string, total any, name, phone string) error
	Header(ctx context.Context, id string) (model.Record, error)
	LockHeader(ctx context.Context, id string) (model.Record, error)
	Registrations(ctx context.Context, cursor, arena, actor string, staff bool, limit int) ([]model.Record, error)
	HeaderItems(ctx context.Context, id string) ([]model.Record, error)
	Payments(ctx context.Context, id string) ([]model.Record, error)
	ClearItems(ctx context.Context, id string) error
	CancelHeader(ctx context.Context, id string) error
	CancelItems(ctx context.Context, id string) error
	CancelPayments(ctx context.Context, id string) error
	CreatePayment(ctx context.Context, arena, reg string, amount any, method, proof string) (model.Record, error)
	Payment(ctx context.Context, id string) (model.Record, error)
	LockPayment(ctx context.Context, id string) (model.Record, error)
	VerifyPayment(ctx context.Context, id, actor string) (model.Record, error)
	RejectPayment(ctx context.Context, id string) (model.Record, error)
	ConfirmItem(ctx context.Context, id, reg, gantangan string) (model.Record, error)
	ConfirmHeader(ctx context.Context, id string) error
}

type Repository interface {
	WorkflowRepository
	Get(context.Context, string, string) (model.Record, error)
	List(context.Context, string, string, string, int) ([]model.Record, error)
	Create(context.Context, string, model.Record) (model.Record, error)
	Update(context.Context, string, string, model.Record) (model.Record, error)
	Delete(context.Context, string, string) error
}
type Authority interface {
	Scope(context.Context, string, string) (model.Scope, error)
	Require(context.Context, string, string, ...string) error
	HasArenaRole(context.Context, string, string, string) (bool, error)
}
type Transactions interface {
	Do(context.Context, func(context.Context) error) error
}
type Notifier interface{ Notify() }
type UseCase struct {
	repo   Repository
	auth   Authority
	tx     Transactions
	notify Notifier
}

func New(r Repository, a Authority, tx Transactions, n Notifier) *UseCase {
	return &UseCase{r, a, tx, n}
}
