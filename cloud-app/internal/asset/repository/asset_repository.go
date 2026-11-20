package repository

import (
	"context"

	"kicau.cloud/control/internal/asset/contract"
	"kicau.cloud/control/internal/asset/model"
	"kicau.cloud/control/internal/infra/postgres"
)

// ---------------- Arenas ----------------

type arenaRepoImpl struct{ store *postgres.Store }

func NewArenaRepository(s *postgres.Store) contract.ArenaRepository {
	return &arenaRepoImpl{store: s}
}

func (r *arenaRepoImpl) GetByID(ctx context.Context, id string) (*model.ArenaView, error) {
	query := `SELECT id, name, address, timezone, is_active, criteria_locked_at, created_at, updated_at FROM cloud_read.v_arenas WHERE id = $1::uuid`
	return postgres.QueryOne[model.ArenaView](ctx, r.store, query, id)
}

func (r *arenaRepoImpl) List(ctx context.Context, cursor *string, limit int) ([]model.ArenaView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `SELECT id, name, address, timezone, is_active, criteria_locked_at, created_at, updated_at FROM cloud_read.v_arenas WHERE ($1::uuid IS NULL OR id > $1::uuid) ORDER BY id LIMIT $2`
	return postgres.QueryMany[model.ArenaView](ctx, r.store, query, cursor, limit)
}

func (r *arenaRepoImpl) Create(ctx context.Context, req *model.CreateArenaRequest) (*model.ArenaView, error) {
	query := `INSERT INTO asset.arenas(name, address, timezone, is_active) VALUES($1, $2, $3, $4) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, req.Name, req.Address, req.Timezone, req.IsActive)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *arenaRepoImpl) Update(ctx context.Context, id string, name, address, timezone *string, isActive *bool) (*model.ArenaView, error) {
	query := `
		UPDATE asset.arenas SET
			name = coalesce($2, name),
			address = coalesce($3, address),
			timezone = coalesce($4, timezone),
			is_active = coalesce($5, is_active),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1::uuid
	`
	err := r.store.ExecSQL(ctx, query, id, name, address, timezone, isActive)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *arenaRepoImpl) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM asset.arenas WHERE id = $1::uuid`
	return r.store.ExecSQL(ctx, query, id)
}

func (r *arenaRepoImpl) Lock(ctx context.Context, id string) (*model.ArenaView, error) {
	query := `SELECT id, name, address, timezone, is_active, criteria_locked_at, created_at, updated_at FROM asset.arenas WHERE id = $1::uuid FOR UPDATE`
	return postgres.QueryOne[model.ArenaView](ctx, r.store, query, id)
}

func (r *arenaRepoImpl) AssignRole(ctx context.Context, userID, arenaID, role string) error {
	query := `INSERT INTO iam.arena_user_roles(user_id, arena_id, role_id) SELECT $1::uuid, $2::uuid, id FROM iam.roles WHERE name = $3 ON CONFLICT (arena_id, user_id, role_id) DO NOTHING`
	return r.store.ExecSQL(ctx, query, userID, arenaID, role)
}

func (r *arenaRepoImpl) ClearAssignments(ctx context.Context, arenaID string) error {
	query := `DELETE FROM iam.arena_user_roles WHERE arena_id = $1::uuid`
	return r.store.ExecSQL(ctx, query, arenaID)
}

// ---------------- Gantangan ----------------

type gantanganRepoImpl struct{ store *postgres.Store }

func NewGantanganRepository(s *postgres.Store) contract.GantanganRepository {
	return &gantanganRepoImpl{store: s}
}

func (r *gantanganRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.GantanganView, error) {
	query := `SELECT id, arena_id, arena_name, number, description, baris, kolom FROM cloud_read.v_gantangan WHERE arena_id = $1::uuid AND id = $2::uuid`
	return postgres.QueryOne[model.GantanganView](ctx, r.store, query, arenaID, id)
}

func (r *gantanganRepoImpl) List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.GantanganView, error) {
	if limit < 1 || limit > 100 {
		limit = 60
	}
	query := `SELECT id, arena_id, arena_name, number, description, baris, kolom FROM cloud_read.v_gantangan WHERE arena_id = $1::uuid AND ($2::uuid IS NULL OR id > $2::uuid) ORDER BY number, id LIMIT $3`
	return postgres.QueryMany[model.GantanganView](ctx, r.store, query, arenaID, cursor, limit)
}

func (r *gantanganRepoImpl) Create(ctx context.Context, arenaID string, number int, desc *string, baris, kolom int) (*model.GantanganView, error) {
	query := `INSERT INTO asset.gantangan(arena_id, number, description, baris, kolom) VALUES($1::uuid, $2, $3, $4, $5) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, arenaID, number, desc, baris, kolom)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *gantanganRepoImpl) Update(ctx context.Context, arenaID, id string, number *int, desc *string, baris, kolom *int) (*model.GantanganView, error) {
	query := `
		UPDATE asset.gantangan SET
			number = coalesce($3, number),
			description = coalesce($4, description),
			baris = coalesce($5, baris),
			kolom = coalesce($6, kolom)
		WHERE arena_id = $1::uuid AND id = $2::uuid
	`
	err := r.store.ExecSQL(ctx, query, arenaID, id, number, desc, baris, kolom)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *gantanganRepoImpl) Delete(ctx context.Context, arenaID, id string) error {
	query := `DELETE FROM asset.gantangan WHERE arena_id = $1::uuid AND id = $2::uuid`
	return r.store.ExecSQL(ctx, query, arenaID, id)
}

// ---------------- Bird Species ----------------

type birdSpeciesRepoImpl struct{ store *postgres.Store }

func NewBirdSpeciesRepository(s *postgres.Store) contract.BirdSpeciesRepository {
	return &birdSpeciesRepoImpl{store: s}
}

func (r *birdSpeciesRepoImpl) GetByID(ctx context.Context, id string) (*model.BirdSpeciesView, error) {
	query := `SELECT id, name, created_at, updated_at FROM cloud_read.v_bird_species WHERE id = $1::uuid`
	return postgres.QueryOne[model.BirdSpeciesView](ctx, r.store, query, id)
}

func (r *birdSpeciesRepoImpl) List(ctx context.Context, cursor *string, limit int) ([]model.BirdSpeciesView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `SELECT id, name, created_at, updated_at FROM cloud_read.v_bird_species WHERE ($1::uuid IS NULL OR id > $1::uuid) ORDER BY name, id LIMIT $2`
	return postgres.QueryMany[model.BirdSpeciesView](ctx, r.store, query, cursor, limit)
}

func (r *birdSpeciesRepoImpl) Create(ctx context.Context, name string) (*model.BirdSpeciesView, error) {
	query := `INSERT INTO asset.bird_species(name) VALUES($1) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, name)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *birdSpeciesRepoImpl) Update(ctx context.Context, id, name string) (*model.BirdSpeciesView, error) {
	query := `UPDATE asset.bird_species SET name = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1::uuid`
	err := r.store.ExecSQL(ctx, query, id, name)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *birdSpeciesRepoImpl) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM asset.bird_species WHERE id = $1::uuid`
	return r.store.ExecSQL(ctx, query, id)
}

// ---------------- Birds ----------------

type birdRepoImpl struct{ store *postgres.Store }

func NewBirdRepository(s *postgres.Store) contract.BirdRepository {
	return &birdRepoImpl{store: s}
}

func (r *birdRepoImpl) SetMutationContext(ctx context.Context, actorID string, arenaID *string) error {
	query := `SELECT set_config('app.actor_id', $1::text, true), set_config('app.arena_id', coalesce($2::text, ''), true)`
	return r.store.ExecSQL(ctx, query, actorID, arenaID)
}

func (r *birdRepoImpl) GetByID(ctx context.Context, id string) (*model.BirdView, error) {
	query := `
		SELECT id, bird_code, name, owner_id, owner_name, bird_species_id, bird_species_name,
		       created_at, updated_at, version, deleted_at
		FROM cloud_read.v_birds
		WHERE id = $1::uuid AND deleted_at IS NULL
	`
	return postgres.QueryOne[model.BirdView](ctx, r.store, query, id)
}

func (r *birdRepoImpl) List(ctx context.Context, ownerID, search, cursor *string, limit int) ([]model.BirdView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, bird_code, name, owner_id, owner_name, bird_species_id, bird_species_name,
		       created_at, updated_at, version, deleted_at
		FROM cloud_read.v_birds v
		WHERE v.deleted_at IS NULL
		  AND ($1::uuid IS NULL OR v.owner_id = $1::uuid)
		  AND ($2::uuid IS NULL OR v.id > $2::uuid)
		  AND ($3::text IS NULL OR strpos(lower(concat_ws(' ', v.name, v.bird_code, v.owner_name, v.bird_species_name)), lower($3)) > 0)
		ORDER BY v.id
		LIMIT $4
	`
	return postgres.QueryMany[model.BirdView](ctx, r.store, query, ownerID, cursor, search, limit)
}

func (r *birdRepoImpl) Create(ctx context.Context, req *model.CreateBirdRequest) (*model.BirdView, error) {
	query := `INSERT INTO asset.birds(name, owner_id, bird_species_id) VALUES($1, $2::uuid, $3::uuid) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, req.Name, req.OwnerID, req.BirdSpeciesID)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *birdRepoImpl) Update(ctx context.Context, id string, req *model.UpdateBirdRequest) (*model.BirdView, error) {
	query := `
		UPDATE asset.birds SET
			name = coalesce($3, name),
			bird_species_id = coalesce($4::uuid, bird_species_id)
		WHERE id = $1::uuid AND version = $2 AND deleted_at IS NULL
	`
	err := r.store.ExecSQL(ctx, query, id, req.ExpectedVersion, req.Name, req.BirdSpeciesID)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *birdRepoImpl) Delete(ctx context.Context, id string, expectedVersion int) error {
	query := `UPDATE asset.birds SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1::uuid AND version = $2`
	return r.store.ExecSQL(ctx, query, id, expectedVersion)
}

func (r *birdRepoImpl) Lock(ctx context.Context, id string, expectedVersion int) (*model.BirdView, error) {
	query := `SELECT id, bird_code, name, owner_id, owner_name, bird_species_id, bird_species_name, created_at, updated_at, version, deleted_at FROM asset.birds WHERE id = $1::uuid AND version = $2 AND deleted_at IS NULL FOR UPDATE`
	return postgres.QueryOne[model.BirdView](ctx, r.store, query, id, expectedVersion)
}

func (r *birdRepoImpl) Transfer(ctx context.Context, id string, expectedVersion int, newOwnerID string, notes *string) error {
	query := `
		WITH updated AS (UPDATE asset.birds SET owner_id = $3::uuid WHERE id = $1::uuid AND version = $2 RETURNING id)
		INSERT INTO asset.bird_ownership_history(bird_id, user_id, notes)
		SELECT id, $3::uuid, $4 FROM updated
	`
	return r.store.ExecSQL(ctx, query, id, expectedVersion, newOwnerID, notes)
}

func (r *birdRepoImpl) OwnershipHistory(ctx context.Context, birdID string) ([]model.BirdOwnershipView, error) {
	query := `SELECT id, bird_id, bird_code, bird_name, user_id, owner_name, ownership_date, notes FROM cloud_read.v_bird_ownership_history WHERE bird_id = $1::uuid ORDER BY ownership_date DESC`
	return postgres.QueryMany[model.BirdOwnershipView](ctx, r.store, query, birdID)
}

func (r *birdRepoImpl) Awards(ctx context.Context, birdID string, cursor *string, limit int) ([]model.AwardView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, bird_id, bird_code, bird_name, registration_id, arena_id, round_id, event_id,
		       award_name, source_type, event_name, organizer_name, rank, awarded_at,
		       has_certificate, created_at, updated_at
		FROM cloud_read.v_bird_award_certificate_history
		WHERE bird_id = $1::uuid AND ($2::uuid IS NULL OR id > $2::uuid)
		ORDER BY id
		LIMIT $3
	`
	return postgres.QueryMany[model.AwardView](ctx, r.store, query, birdID, cursor, limit)
}

func (r *birdRepoImpl) Award(ctx context.Context, birdID, awardID string) (*model.AwardView, error) {
	query := `
		SELECT id, bird_id, bird_code, bird_name, registration_id, arena_id, round_id, event_id,
		       award_name, source_type, event_name, organizer_name, rank, awarded_at,
		       has_certificate, created_at, updated_at
		FROM cloud_read.v_bird_award_certificate_history
		WHERE bird_id = $1::uuid AND id = $2::uuid
	`
	return postgres.QueryOne[model.AwardView](ctx, r.store, query, birdID, awardID)
}

// ---------------- Categories ----------------

type categoryRepoImpl struct{ store *postgres.Store }

func NewCategoryRepository(s *postgres.Store) contract.CategoryRepository {
	return &categoryRepoImpl{store: s}
}

func (r *categoryRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.CategoryView, error) {
	query := `SELECT id, arena_id, code, name FROM cloud_read.v_categories WHERE arena_id = $1::uuid AND id = $2::uuid`
	return postgres.QueryOne[model.CategoryView](ctx, r.store, query, arenaID, id)
}

func (r *categoryRepoImpl) List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.CategoryView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `SELECT id, arena_id, code, name FROM cloud_read.v_categories WHERE arena_id = $1::uuid AND ($2::uuid IS NULL OR id > $2::uuid) ORDER BY code, id LIMIT $3`
	return postgres.QueryMany[model.CategoryView](ctx, r.store, query, arenaID, cursor, limit)
}

func (r *categoryRepoImpl) Create(ctx context.Context, arenaID, code, name string) (*model.CategoryView, error) {
	query := `INSERT INTO competition.categories(arena_id, code, name) VALUES($1::uuid, $2, $3) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, arenaID, code, name)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *categoryRepoImpl) Update(ctx context.Context, arenaID, id string, code, name *string) (*model.CategoryView, error) {
	query := `
		UPDATE competition.categories SET
			code = coalesce($3, code),
			name = coalesce($4, name)
		WHERE arena_id = $1::uuid AND id = $2::uuid
	`
	err := r.store.ExecSQL(ctx, query, arenaID, id, code, name)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *categoryRepoImpl) Delete(ctx context.Context, arenaID, id string) error {
	query := `DELETE FROM competition.categories WHERE arena_id = $1::uuid AND id = $2::uuid`
	return r.store.ExecSQL(ctx, query, arenaID, id)
}

// ---------------- Criteria ----------------

type criteriaRepoImpl struct{ store *postgres.Store }

func NewCriteriaRepository(s *postgres.Store) contract.CriteriaRepository {
	return &criteriaRepoImpl{store: s}
}

func (r *criteriaRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.ScoreCriteriaView, error) {
	query := `SELECT id, arena_id, criteria_name FROM cloud_read.v_score_criteria WHERE arena_id = $1::uuid AND id = $2::uuid`
	return postgres.QueryOne[model.ScoreCriteriaView](ctx, r.store, query, arenaID, id)
}

func (r *criteriaRepoImpl) List(ctx context.Context, arenaID string) ([]model.ScoreCriteriaView, error) {
	query := `SELECT id, arena_id, criteria_name FROM cloud_read.v_score_criteria WHERE arena_id = $1::uuid ORDER BY criteria_name, id`
	return postgres.QueryMany[model.ScoreCriteriaView](ctx, r.store, query, arenaID)
}

func (r *criteriaRepoImpl) Create(ctx context.Context, arenaID, criteriaName string) (*model.ScoreCriteriaView, error) {
	query := `INSERT INTO judging.score_criteria(arena_id, criteria_name) VALUES($1::uuid, $2) RETURNING id`
	row, err := r.store.QueryRow(ctx, query, arenaID, criteriaName)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *criteriaRepoImpl) Update(ctx context.Context, arenaID, id string, criteriaName string) (*model.ScoreCriteriaView, error) {
	query := `UPDATE judging.score_criteria SET criteria_name = $3 WHERE arena_id = $1::uuid AND id = $2::uuid`
	err := r.store.ExecSQL(ctx, query, arenaID, id, criteriaName)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *criteriaRepoImpl) Delete(ctx context.Context, arenaID, id string) error {
	query := `DELETE FROM judging.score_criteria WHERE arena_id = $1::uuid AND id = $2::uuid`
	return r.store.ExecSQL(ctx, query, arenaID, id)
}

func (r *criteriaRepoImpl) LockCriteria(ctx context.Context, arenaID string) error {
	query := `UPDATE asset.arenas SET criteria_locked_at = coalesce(criteria_locked_at, CURRENT_TIMESTAMP) WHERE id = $1::uuid`
	return r.store.ExecSQL(ctx, query, arenaID)
}
