package model

import "time"

// ArenaView represents a row projected from cloud_read.v_arenas.
type ArenaView struct {
	ID               string     `db:"id" json:"id"`
	Name             string     `db:"name" json:"name"`
	Address          *string    `db:"address" json:"address"`
	Timezone         string     `db:"timezone" json:"timezone"`
	IsActive         bool       `db:"is_active" json:"is_active"`
	CriteriaLockedAt *time.Time `db:"criteria_locked_at" json:"criteria_locked_at,omitempty"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
}

// GantanganView represents a row projected from cloud_read.v_gantangan.
type GantanganView struct {
	ID          string  `db:"id" json:"id"`
	ArenaID     string  `db:"arena_id" json:"arena_id"`
	ArenaName   string  `db:"arena_name" json:"arena_name"`
	Number      int     `db:"number" json:"number"`
	Description *string `db:"description" json:"description"`
	Baris       int     `db:"baris" json:"baris"`
	Kolom       int     `db:"kolom" json:"kolom"`
}

// BirdSpeciesView represents a row projected from cloud_read.v_bird_species.
type BirdSpeciesView struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// BirdView represents a row projected from cloud_read.v_birds.
type BirdView struct {
	ID              string     `db:"id" json:"id"`
	BirdCode        string     `db:"bird_code" json:"bird_code"`
	Name            string     `db:"name" json:"name"`
	OwnerID         string     `db:"owner_id" json:"owner_id"`
	OwnerName       string     `db:"owner_name" json:"owner_name"`
	BirdSpeciesID   string     `db:"bird_species_id" json:"bird_species_id"`
	BirdSpeciesName string     `db:"bird_species_name" json:"bird_species_name"`
	CreatedAt       time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at" json:"updated_at"`
	Version         int        `db:"version" json:"version"`
	DeletedAt       *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

// CategoryView represents a row projected from cloud_read.v_categories.
type CategoryView struct {
	ID      string `db:"id" json:"id"`
	ArenaID string `db:"arena_id" json:"arena_id"`
	Code    string `db:"code" json:"code"`
	Name    string `db:"name" json:"name"`
}

// ScoreCriteriaView represents a row projected from cloud_read.v_score_criteria.
type ScoreCriteriaView struct {
	ID           string `db:"id" json:"id"`
	ArenaID      string `db:"arena_id" json:"arena_id"`
	CriteriaName string `db:"criteria_name" json:"criteria_name"`
}

// AwardView represents a row projected from cloud_read.v_bird_award_certificate_history.
type AwardView struct {
	ID             string     `db:"id" json:"id"`
	BirdID         string     `db:"bird_id" json:"bird_id"`
	BirdCode       string     `db:"bird_code" json:"bird_code"`
	BirdName       string     `db:"bird_name" json:"bird_name"`
	RegistrationID *string    `db:"registration_id" json:"registration_id,omitempty"`
	ArenaID        *string    `db:"arena_id" json:"arena_id,omitempty"`
	RoundID        *string    `db:"round_id" json:"round_id,omitempty"`
	EventID        *string    `db:"event_id" json:"event_id,omitempty"`
	AwardName      string     `db:"award_name" json:"award_name"`
	SourceType     string     `db:"source_type" json:"source_type"`
	EventName      *string    `db:"event_name" json:"event_name,omitempty"`
	OrganizerName  *string    `db:"organizer_name" json:"organizer_name,omitempty"`
	Rank           *int       `db:"rank" json:"rank,omitempty"`
	AwardedAt      *time.Time `db:"awarded_at" json:"awarded_at,omitempty"`
	HasCertificate bool       `db:"has_certificate" json:"has_certificate"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at" json:"updated_at"`
}

// BirdOwnershipView represents a row projected from cloud_read.v_bird_ownership_history.
type BirdOwnershipView struct {
	ID            string    `db:"id" json:"id"`
	BirdID        string    `db:"bird_id" json:"bird_id"`
	BirdCode      string    `db:"bird_code" json:"bird_code"`
	BirdName      string    `db:"bird_name" json:"bird_name"`
	UserID        string    `db:"user_id" json:"user_id"`
	OwnerName     string    `db:"owner_name" json:"owner_name"`
	OwnershipDate time.Time `db:"ownership_date" json:"ownership_date"`
	Notes         *string   `db:"notes" json:"notes,omitempty"`
}

// CreateArenaRequest DTO
type CreateArenaRequest struct {
	Name     string  `json:"name"`
	Address  *string `json:"address"`
	Timezone string  `json:"timezone"`
	IsActive bool    `json:"is_active"`
}

// CreateBirdRequest DTO
type CreateBirdRequest struct {
	Name          string `json:"name"`
	OwnerID       string `json:"owner_id"`
	BirdSpeciesID string `json:"bird_species_id"`
}

// UpdateBirdRequest DTO
type UpdateBirdRequest struct {
	ExpectedVersion int     `json:"expected_version"`
	Name            *string `json:"name,omitempty"`
	BirdSpeciesID   *string `json:"bird_species_id,omitempty"`
}

// TransferBirdRequest DTO
type TransferBirdRequest struct {
	ExpectedVersion int     `json:"expected_version"`
	OwnerID         string  `json:"owner_id"`
	Notes           *string `json:"notes,omitempty"`
}
