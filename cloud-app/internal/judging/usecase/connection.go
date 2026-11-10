// Package usecase owns scoring rules, revisions and the shared arena criteria list.
package usecase

import (
	"context"

	"kicau.cloud/control/internal/core/model"
)

type WorkflowRepository interface {
	Arena(ctx context.Context, id string) (model.Record, error)
	LockArena(ctx context.Context, id string) (model.Record, error)
	Round(ctx context.Context, id string) (model.Record, error)
	LockRound(ctx context.Context, id string) (model.Record, error)
	Criteria(ctx context.Context, arena string) ([]model.Record, error)
	Assignment(ctx context.Context, round, actor string) (model.Record, error)
	Rounds(ctx context.Context, actor, cursor string, limit int) ([]model.Record, error)
	Item(ctx context.Context, id string) (model.Record, error)
	Items(ctx context.Context, round string) ([]model.Record, error)
	Assessment(ctx context.Context, item, judge string) (model.Record, error)
	SaveAssessment(ctx context.Context, arena, round, item, judge string, version int) (model.Record, error)
	Scores(ctx context.Context, item, judge string) ([]model.Record, error)
	History(ctx context.Context, item, judge string) ([]model.Record, error)
	Score(ctx context.Context, arena, round, item, judge, criterion string, value, version int) (model.Record, error)
	Revision(ctx context.Context, id, operation string, version, value int) error
	Operation(ctx context.Context, actor, operation string) (model.Record, error)
	SaveOperation(ctx context.Context, actor, operation string, hash []byte, assessment string, result string) error
}

type Repository interface {
	WorkflowRepository
	Get(context.Context, string, string) (model.Record, error)
	List(context.Context, string, string, string, int) ([]model.Record, error)
	Create(context.Context, string, model.Record) (model.Record, error)
	Update(context.Context, string, string, model.Record) (model.Record, error)
	Delete(context.Context, string, string) error
}
type Transactions interface {
	Do(context.Context, func(context.Context) error) error
}
type Authority interface {
	Require(context.Context, string, string, ...string) error
	HasArenaRole(context.Context, string, string, string) (bool, error)
}
type UseCase struct {
	repo Repository
	auth Authority
	tx   Transactions
}

func New(r Repository, a Authority, tx Transactions) *UseCase { return &UseCase{r, a, tx} }
