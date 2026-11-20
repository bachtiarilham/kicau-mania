package model

import (
	"time"
)

// UserView represents a row projected from cloud_read.v_users.
type UserView struct {
	ID              string     `db:"id" json:"id"`
	Name            string     `db:"name" json:"name"`
	Phone           *string    `db:"phone" json:"phone"`
	Email           *string    `db:"email" json:"email"`
	EmailVerifiedAt *time.Time `db:"email_verified_at" json:"email_verified_at,omitempty"`
	IsActive        bool       `db:"is_active" json:"is_active"`
	IsEO            bool       `db:"is_eo" json:"is_eo"`
	HasPassword     bool       `db:"has_password" json:"has_password"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	Version         int        `db:"version" json:"version"`
	DeletedAt       *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
	Nickname        *string    `db:"nickname" json:"nickname"`
	Address         *string    `db:"address" json:"address"`
	Position        *string    `db:"position" json:"position"`
}

// UserProfile represents the extended profile returned to frontend clients.
type UserProfile struct {
	UserView
	Roles      []string         `json:"roles"`
	ArenaRoles []ArenaRoleScope `json:"arena_roles"`
}

// ArenaRoleScope represents a role scoped to an arena.
type ArenaRoleScope struct {
	ArenaID string `db:"arena_id" json:"arena_id"`
	Role    string `db:"role" json:"role"`
}

// CreateUserRequest describes the request payload to register or create an account.
type CreateUserRequest struct {
	Name     string  `json:"name"`
	Phone    *string `json:"phone"`
	Email    *string `json:"email"`
	Password *string `json:"password,omitempty"`
	Nickname *string `json:"nickname,omitempty"`
	Address  *string `json:"address,omitempty"`
	Position *string `json:"position,omitempty"`
	IsEO     bool    `json:"is_eo"`
	IsActive bool    `json:"is_active"`
}

// UpdateUserRequest describes the patch payload to update an existing user.
type UpdateUserRequest struct {
	ExpectedVersion int     `json:"expected_version"`
	Name            *string `json:"name,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	Email           *string `json:"email,omitempty"`
	Nickname        *string `json:"nickname,omitempty"`
	Address         *string `json:"address,omitempty"`
	Position        *string `json:"position,omitempty"`
	IsEO            *bool   `json:"is_eo,omitempty"`
	IsActive        *bool   `json:"is_active,omitempty"`
}

// UserFilter represents pagination and search filters for listing users.
type UserFilter struct {
	Cursor *string
	Limit  int
	IsEO   *bool
	Role   *string
	Search *string
}
