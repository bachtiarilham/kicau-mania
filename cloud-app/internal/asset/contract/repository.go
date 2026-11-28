package contract

import (
	"context"

	"kicau.cloud/control/internal/asset/model"
)

type TransactionManager interface {
	Do(ctx context.Context, f func(context.Context) error) error
}

type ArenaRepository interface {
	GetByID(ctx context.Context, id string) (*model.ArenaView, error)
	List(ctx context.Context, cursor *string, limit int) ([]model.ArenaView, error)
	Create(ctx context.Context, req *model.CreateArenaRequest) (*model.ArenaView, error)
	Update(ctx context.Context, id string, name *string, address *string, timezone *string, isActive *bool) (*model.ArenaView, error)
	Delete(ctx context.Context, id string) error
	Lock(ctx context.Context, id string) (*model.ArenaView, error)
	AssignRole(ctx context.Context, userID, arenaID, role string) error
	ClearAssignments(ctx context.Context, arenaID string) error
}

type GantanganRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.GantanganView, error)
	List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.GantanganView, error)
	Create(ctx context.Context, arenaID string, number int, desc *string, baris, kolom int) (*model.GantanganView, error)
	Update(ctx context.Context, arenaID, id string, number *int, desc *string, baris, kolom *int) (*model.GantanganView, error)
	Delete(ctx context.Context, arenaID, id string) error
}

type BirdSpeciesRepository interface {
	GetByID(ctx context.Context, id string) (*model.BirdSpeciesView, error)
	List(ctx context.Context, cursor *string, limit int) ([]model.BirdSpeciesView, error)
	Create(ctx context.Context, req *model.BirdSpeciesRequest) (*model.BirdSpeciesView, error)
	Update(ctx context.Context, id string, req *model.BirdSpeciesRequest) (*model.BirdSpeciesView, error)
	Delete(ctx context.Context, id string) error
}

type BirdRepository interface {
	GetByID(ctx context.Context, id string) (*model.BirdView, error)
	List(ctx context.Context, ownerID *string, search *string, cursor *string, limit int) ([]model.BirdView, error)
	Create(ctx context.Context, req *model.CreateBirdRequest) (*model.BirdView, error)
	Update(ctx context.Context, id string, req *model.UpdateBirdRequest) (*model.BirdView, error)
	Delete(ctx context.Context, id string, expectedVersion int) error
	Lock(ctx context.Context, id string, expectedVersion int) (*model.BirdView, error)
	Transfer(ctx context.Context, id string, expectedVersion int, newOwnerID string, notes *string) error
	OwnershipHistory(ctx context.Context, birdID string) ([]model.BirdOwnershipView, error)
	Awards(ctx context.Context, birdID string, cursor *string, limit int) ([]model.AwardView, error)
	Award(ctx context.Context, birdID, awardID string) (*model.AwardView, error)
	SetMutationContext(ctx context.Context, actorID string, arenaID *string) error
}

type CategoryRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.CategoryView, error)
	List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.CategoryView, error)
	Create(ctx context.Context, arenaID, code, name string) (*model.CategoryView, error)
	Update(ctx context.Context, arenaID, id string, code, name *string) (*model.CategoryView, error)
	Delete(ctx context.Context, arenaID, id string) error
}

type CriteriaRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.ScoreCriteriaView, error)
	List(ctx context.Context, arenaID string) ([]model.ScoreCriteriaView, error)
	Create(ctx context.Context, arenaID, criteriaName string) (*model.ScoreCriteriaView, error)
	Update(ctx context.Context, arenaID, id string, criteriaName string) (*model.ScoreCriteriaView, error)
	Delete(ctx context.Context, arenaID, id string) error
	LockCriteria(ctx context.Context, arenaID string) error
}
