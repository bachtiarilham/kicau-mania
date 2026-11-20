package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/identity/contract"
	"kicau.cloud/control/internal/identity/model"
	"kicau.cloud/control/internal/infra/postgres"
)

type userRepositoryImpl struct {
	store *postgres.Store
}

func NewUserRepository(s *postgres.Store) contract.UserRepository {
	return &userRepositoryImpl{store: s}
}

func (r *userRepositoryImpl) SetMutationContext(ctx context.Context, actorID string, arenaID *string) error {
	query := `SELECT set_config('app.actor_id', $1::text, true), set_config('app.arena_id', coalesce($2::text, ''), true)`
	return r.store.ExecSQL(ctx, query, actorID, arenaID)
}

func (r *userRepositoryImpl) BootstrapLock(ctx context.Context) error {
	return r.store.ExecSQL(ctx, "SELECT pg_advisory_xact_lock(753125)")
}

func (r *userRepositoryImpl) AdminExists(ctx context.Context) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM iam.user_roles a JOIN iam.roles r ON r.id = a.role_id WHERE r.name = 'ADMIN')`
	row, err := r.store.QueryRow(ctx, query)
	if err != nil {
		return false, err
	}
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *userRepositoryImpl) VerifyEmail(ctx context.Context, id string) error {
	query := `UPDATE iam.users SET email_verified_at = CURRENT_TIMESTAMP WHERE id = $1::uuid AND email_verified_at IS NULL`
	return r.store.ExecSQL(ctx, query, id)
}

func (r *userRepositoryImpl) AssignGlobalRole(ctx context.Context, id, role string) error {
	query := `INSERT INTO iam.user_roles(user_id, role_id) SELECT $1::uuid, id FROM iam.roles WHERE name = $2 ON CONFLICT DO NOTHING`
	return r.store.ExecSQL(ctx, query, id, role)
}


func (r *userRepositoryImpl) GetByID(ctx context.Context, id string) (*model.UserView, error) {
	query := `
		SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
		       has_password, created_at, updated_at, version, deleted_at,
		       nickname, address, position
		FROM cloud_read.v_users
		WHERE id = $1::uuid AND deleted_at IS NULL
	`
	return postgres.QueryOne[model.UserView](ctx, r.store, query, id)
}

func (r *userRepositoryImpl) GetByEmail(ctx context.Context, email string) (*model.UserView, error) {
	query := `
		SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
		       has_password, created_at, updated_at, version, deleted_at,
		       nickname, address, position
		FROM cloud_read.v_users
		WHERE lower(btrim(email)) = lower(btrim($1)) AND deleted_at IS NULL
	`
	return postgres.QueryOne[model.UserView](ctx, r.store, query, email)
}

func (r *userRepositoryImpl) GetProfile(ctx context.Context, id string) (*model.UserProfile, error) {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	profile := &model.UserProfile{
		UserView:   *user,
		Roles:      []string{},
		ArenaRoles: []model.ArenaRoleScope{},
	}

	rolesQuery := `
		SELECT r.name
		FROM iam.user_roles u
		JOIN iam.roles r ON r.id = u.role_id
		WHERE u.user_id = $1::uuid
		ORDER BY r.name
	`
	row, err := r.store.QueryRow(ctx, "SELECT coalesce((SELECT jsonb_agg(r.name ORDER BY r.name) FROM iam.user_roles u JOIN iam.roles r ON r.id=u.role_id WHERE u.user_id=$1::uuid),'[]'::jsonb), coalesce((SELECT jsonb_agg(jsonb_build_object('arena_id',a.arena_id,'role',r.name) ORDER BY a.arena_id,r.name) FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::uuid),'[]'::jsonb)", id)
	if err != nil {
		return nil, err
	}
	var rolesJSON, arenaRolesJSON []byte
	if err := row.Scan(&rolesJSON, &arenaRolesJSON); err == nil {
		_ = rolesQuery // fallback
	}

	type roleName struct {
		Name string `db:"name"`
	}
	roles, _ := postgres.QueryMany[roleName](ctx, r.store, rolesQuery, id)
	for _, rn := range roles {
		profile.Roles = append(profile.Roles, rn.Name)
	}

	arenaRolesQuery := `
		SELECT a.arena_id, r.name AS role
		FROM iam.arena_user_roles a
		JOIN iam.roles r ON r.id = a.role_id
		WHERE a.user_id = $1::uuid
		ORDER BY a.arena_id, r.name
	`
	arenaRoles, _ := postgres.QueryMany[model.ArenaRoleScope](ctx, r.store, arenaRolesQuery, id)
	profile.ArenaRoles = arenaRoles

	return profile, nil
}

func (r *userRepositoryImpl) List(ctx context.Context, filter model.UserFilter) ([]model.UserView, error) {
	limit := filter.Limit
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
		       has_password, created_at, updated_at, version, deleted_at,
		       nickname, address, position
		FROM cloud_read.v_users v
		WHERE v.deleted_at IS NULL
		  AND ($1::uuid IS NULL OR v.id > $1::uuid)
		  AND ($2::boolean IS NULL OR v.is_eo = $2)
		  AND ($3::text IS NULL OR EXISTS(SELECT 1 FROM iam.user_roles a JOIN iam.roles r ON r.id = a.role_id WHERE a.user_id = v.id AND r.name = $3) OR EXISTS(SELECT 1 FROM iam.arena_user_roles a JOIN iam.roles r ON r.id = a.role_id WHERE a.user_id = v.id AND r.name = $3))
		  AND ($4::text IS NULL OR strpos(lower(concat_ws(' ', v.name, v.phone, v.email, v.nickname)), lower($4)) > 0)
		ORDER BY v.id
		LIMIT $5
	`
	return postgres.QueryMany[model.UserView](ctx, r.store, query, filter.Cursor, filter.IsEO, filter.Role, filter.Search, limit)
}

func (r *userRepositoryImpl) Create(ctx context.Context, id string, req *model.CreateUserRequest, passwordHash *string) (*model.UserView, error) {
	insertQuery := `
		INSERT INTO iam.users(id, name, phone, email, password_hash, is_eo, is_active, nickname, address, position)
		VALUES($1::uuid, $2, $3, lower(btrim($4)), $5, $6, $7, $8, $9, $10)
	`
	err := r.store.ExecSQL(ctx, insertQuery, id, req.Name, req.Phone, req.Email, passwordHash, req.IsEO, req.IsActive, req.Nickname, req.Address, req.Position)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepositoryImpl) Update(ctx context.Context, id string, req *model.UpdateUserRequest) (*model.UserView, error) {
	updateQuery := `
		UPDATE iam.users SET
			name = coalesce($3, name),
			phone = coalesce($4, phone),
			email = CASE WHEN $5::text IS NOT NULL THEN lower(btrim($5)) ELSE email END,
			nickname = coalesce($6, nickname),
			address = coalesce($7, address),
			position = coalesce($8, position),
			is_eo = coalesce($9, is_eo),
			is_active = coalesce($10, is_active)
		WHERE id = $1::uuid AND version = $2 AND deleted_at IS NULL
	`
	err := r.store.ExecSQL(ctx, updateQuery, id, req.ExpectedVersion, req.Name, req.Phone, req.Email, req.Nickname, req.Address, req.Position, req.IsEO, req.IsActive)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *userRepositoryImpl) Delete(ctx context.Context, id string, expectedVersion int) error {
	query := `UPDATE iam.users SET deleted_at = CURRENT_TIMESTAMP WHERE id = $1::uuid AND version = $2`
	return r.store.ExecSQL(ctx, query, id, expectedVersion)
}

func (r *userRepositoryImpl) GetPasswordHash(ctx context.Context, id string) (string, error) {
	query := `SELECT password_hash FROM iam.users WHERE id = $1::uuid AND deleted_at IS NULL`
	row, err := r.store.QueryRow(ctx, query, id)
	if err != nil {
		return "", err
	}
	var hash *string
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperror.ErrNotFound
		}
		return "", err
	}
	if hash == nil {
		return "", nil
	}
	return *hash, nil
}

func (r *userRepositoryImpl) UpdatePassword(ctx context.Context, id string, newHash string) error {
	query := `UPDATE iam.users SET password_hash = $2 WHERE id = $1::uuid AND deleted_at IS NULL`
	return r.store.ExecSQL(ctx, query, id, newHash)
}
