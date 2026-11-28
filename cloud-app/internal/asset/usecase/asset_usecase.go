package usecase

import (
	"context"
	"strings"
	"time"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/asset/contract"
	"kicau.cloud/control/internal/asset/model"
)

type AssetUsecase struct {
	tx       contract.TransactionManager
	arenas   contract.ArenaRepository
	gantang  contract.GantanganRepository
	species  contract.BirdSpeciesRepository
	birds    contract.BirdRepository
	cats     contract.CategoryRepository
	criteria contract.CriteriaRepository
}

func NewAssetUsecase(
	tx contract.TransactionManager,
	arenas contract.ArenaRepository,
	gantang contract.GantanganRepository,
	species contract.BirdSpeciesRepository,
	birds contract.BirdRepository,
	cats contract.CategoryRepository,
	criteria contract.CriteriaRepository,
) *AssetUsecase {
	return &AssetUsecase{
		tx:       tx,
		arenas:   arenas,
		gantang:  gantang,
		species:  species,
		birds:    birds,
		cats:     cats,
		criteria: criteria,
	}
}

// ---------------- Arenas ----------------

func (u *AssetUsecase) GetArena(ctx context.Context, id string) (*model.ArenaView, error) {
	return u.arenas.GetByID(ctx, id)
}

func (u *AssetUsecase) ListArenas(ctx context.Context, cursor *string, limit int) ([]model.ArenaView, error) {
	return u.arenas.List(ctx, cursor, limit)
}

func (u *AssetUsecase) CreateArena(ctx context.Context, actorID string, req *model.CreateArenaRequest) (*model.ArenaView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.Invalid("Nama arena wajib diisi")
	}
	if req.Timezone == "" {
		req.Timezone = "Asia/Jakarta"
	}
	if _, err := time.LoadLocation(req.Timezone); err != nil {
		return nil, apperror.Invalid("Zona waktu tidak valid")
	}

	var created *model.ArenaView
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		a, err := u.arenas.Create(txCtx, req)
		if err != nil {
			return err
		}
		if err := u.arenas.AssignRole(txCtx, actorID, a.ID, "ADMIN"); err != nil {
			return err
		}
		created = a
		return nil
	})
	return created, err
}

func (u *AssetUsecase) UpdateArena(ctx context.Context, id string, name, address, timezone *string, isActive *bool) (*model.ArenaView, error) {
	if timezone != nil && *timezone != "" {
		if _, err := time.LoadLocation(*timezone); err != nil {
			return nil, apperror.Invalid("Zona waktu tidak valid")
		}
	}
	return u.arenas.Update(ctx, id, name, address, timezone, isActive)
}

func (u *AssetUsecase) DeleteArena(ctx context.Context, id string) error {
	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.arenas.ClearAssignments(txCtx, id); err != nil {
			return err
		}
		return u.arenas.Delete(txCtx, id)
	})
}

// ---------------- Gantangan ----------------

func (u *AssetUsecase) GetGantangan(ctx context.Context, arenaID, id string) (*model.GantanganView, error) {
	return u.gantang.GetByID(ctx, arenaID, id)
}

func (u *AssetUsecase) ListGantangan(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.GantanganView, error) {
	return u.gantang.List(ctx, arenaID, cursor, limit)
}

func (u *AssetUsecase) CreateGantangan(ctx context.Context, arenaID string, number int, desc *string, baris, kolom int) (*model.GantanganView, error) {
	if number < 1 || baris < 1 || kolom < 1 {
		return nil, apperror.Invalid("Nomor, baris, dan kolom gantangan harus positif")
	}
	return u.gantang.Create(ctx, arenaID, number, desc, baris, kolom)
}

func (u *AssetUsecase) UpdateGantangan(ctx context.Context, arenaID, id string, number *int, desc *string, baris, kolom *int) (*model.GantanganView, error) {
	return u.gantang.Update(ctx, arenaID, id, number, desc, baris, kolom)
}

func (u *AssetUsecase) DeleteGantangan(ctx context.Context, arenaID, id string) error {
	return u.gantang.Delete(ctx, arenaID, id)
}

// ---------------- Bird Species ----------------

func (u *AssetUsecase) GetSpecies(ctx context.Context, id string) (*model.BirdSpeciesView, error) {
	return u.species.GetByID(ctx, id)
}

func (u *AssetUsecase) ListSpecies(ctx context.Context, cursor *string, limit int) ([]model.BirdSpeciesView, error) {
	return u.species.List(ctx, cursor, limit)
}

func (u *AssetUsecase) CreateSpecies(ctx context.Context, req *model.BirdSpeciesRequest) (*model.BirdSpeciesView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.Invalid("Nama jenis burung wajib diisi")
	}
	return u.species.Create(ctx, req)
}

func (u *AssetUsecase) UpdateSpecies(ctx context.Context, id string, req *model.BirdSpeciesRequest) (*model.BirdSpeciesView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.Invalid("Nama jenis burung tidak boleh kosong")
	}
	return u.species.Update(ctx, id, req)
}

func (u *AssetUsecase) DeleteSpecies(ctx context.Context, id string) error {
	return u.species.Delete(ctx, id)
}

// ---------------- Birds ----------------

func (u *AssetUsecase) GetBird(ctx context.Context, id string) (*model.BirdView, error) {
	return u.birds.GetByID(ctx, id)
}

func (u *AssetUsecase) ListBirds(ctx context.Context, ownerID, search, cursor *string, limit int) ([]model.BirdView, error) {
	return u.birds.List(ctx, ownerID, search, cursor, limit)
}

func (u *AssetUsecase) CreateBird(ctx context.Context, actorID string, req *model.CreateBirdRequest) (*model.BirdView, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, apperror.Invalid("Nama burung wajib diisi")
	}
	if req.OwnerID == "" || req.BirdSpeciesID == "" {
		return nil, apperror.Invalid("Pemilik dan jenis burung wajib diisi")
	}

	var created *model.BirdView
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.birds.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		b, err := u.birds.Create(txCtx, req)
		if err != nil {
			return err
		}
		created = b
		return nil
	})
	return created, err
}

func (u *AssetUsecase) UpdateBird(ctx context.Context, actorID, id string, req *model.UpdateBirdRequest) (*model.BirdView, error) {
	if req.ExpectedVersion < 1 {
		return nil, apperror.Invalid("expected_version wajib diisi")
	}

	var updated *model.BirdView
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.birds.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		if _, err := u.birds.Lock(txCtx, id, req.ExpectedVersion); err != nil {
			return err
		}
		b, err := u.birds.Update(txCtx, id, req)
		if err != nil {
			return err
		}
		updated = b
		return nil
	})
	return updated, err
}

func (u *AssetUsecase) DeleteBird(ctx context.Context, actorID, id string, expectedVersion int) error {
	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.birds.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		if _, err := u.birds.Lock(txCtx, id, expectedVersion); err != nil {
			return err
		}
		return u.birds.Delete(txCtx, id, expectedVersion)
	})
}

func (u *AssetUsecase) TransferBird(ctx context.Context, actorID, id string, req *model.TransferBirdRequest) error {
	if req.ExpectedVersion < 1 {
		return apperror.Invalid("expected_version wajib diisi")
	}
	if req.OwnerID == "" {
		return apperror.Invalid("Pemilik baru wajib diisi")
	}

	return u.tx.Do(ctx, func(txCtx context.Context) error {
		if err := u.birds.SetMutationContext(txCtx, actorID, nil); err != nil {
			return err
		}
		if _, err := u.birds.Lock(txCtx, id, req.ExpectedVersion); err != nil {
			return err
		}
		return u.birds.Transfer(txCtx, id, req.ExpectedVersion, req.OwnerID, req.Notes)
	})
}

func (u *AssetUsecase) BirdOwnership(ctx context.Context, birdID string) ([]model.BirdOwnershipView, error) {
	return u.birds.OwnershipHistory(ctx, birdID)
}

func (u *AssetUsecase) BirdAwards(ctx context.Context, birdID string, cursor *string, limit int) ([]model.AwardView, error) {
	return u.birds.Awards(ctx, birdID, cursor, limit)
}

func (u *AssetUsecase) BirdAward(ctx context.Context, birdID, awardID string) (*model.AwardView, error) {
	return u.birds.Award(ctx, birdID, awardID)
}

// ---------------- Categories ----------------

func (u *AssetUsecase) GetCategory(ctx context.Context, arenaID, id string) (*model.CategoryView, error) {
	return u.cats.GetByID(ctx, arenaID, id)
}

func (u *AssetUsecase) ListCategories(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.CategoryView, error) {
	return u.cats.List(ctx, arenaID, cursor, limit)
}

func (u *AssetUsecase) CreateCategory(ctx context.Context, arenaID, code, name string) (*model.CategoryView, error) {
	if strings.TrimSpace(code) == "" || strings.TrimSpace(name) == "" {
		return nil, apperror.Invalid("Kode dan nama kategori wajib diisi")
	}
	return u.cats.Create(ctx, arenaID, code, name)
}

func (u *AssetUsecase) UpdateCategory(ctx context.Context, arenaID, id string, code, name *string) (*model.CategoryView, error) {
	return u.cats.Update(ctx, arenaID, id, code, name)
}

func (u *AssetUsecase) DeleteCategory(ctx context.Context, arenaID, id string) error {
	return u.cats.Delete(ctx, arenaID, id)
}

// ---------------- Criteria ----------------

func (u *AssetUsecase) ListCriteria(ctx context.Context, arenaID string) ([]model.ScoreCriteriaView, error) {
	return u.criteria.List(ctx, arenaID)
}

func (u *AssetUsecase) CreateCriteria(ctx context.Context, arenaID, name string) (*model.ScoreCriteriaView, error) {
	if strings.TrimSpace(name) == "" {
		return nil, apperror.Invalid("Nama kriteria wajib diisi")
	}
	return u.criteria.Create(ctx, arenaID, name)
}

func (u *AssetUsecase) UpdateCriteria(ctx context.Context, arenaID, id, name string) (*model.ScoreCriteriaView, error) {
	return u.criteria.Update(ctx, arenaID, id, name)
}

func (u *AssetUsecase) DeleteCriteria(ctx context.Context, arenaID, id string) error {
	return u.criteria.Delete(ctx, arenaID, id)
}
