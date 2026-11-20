package model

import "time"

// EventView represents a row projected from cloud_read.v_events.
type EventView struct {
	ID              string    `db:"id" json:"id"`
	ArenaID         string    `db:"arena_id" json:"arena_id"`
	ArenaName       string    `db:"arena_name" json:"arena_name"`
	OwnerID         string    `db:"owner_id" json:"owner_id"`
	OrganizerName   string    `db:"organizer_name" json:"organizer_name"`
	Title           string    `db:"title" json:"title"`
	Description     *string   `db:"description" json:"description"`
	EventDate       time.Time `db:"event_date" json:"event_date"`
	Status          string    `db:"status" json:"status"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
	RoundCount      int64     `db:"round_count" json:"round_count"`
	Location        *string   `db:"location" json:"location,omitempty"`
	GrandPrize      *string   `db:"grand_prize" json:"grand_prize,omitempty"`
	PicName         *string   `db:"pic_name" json:"pic_name,omitempty"`
	MinParticipants int       `db:"min_participants" json:"min_participants"`
	MaxParticipants int       `db:"max_participants" json:"max_participants"`
}

// RoundView represents a row projected from cloud_read.v_rounds.
type RoundView struct {
	ID                     string    `db:"id" json:"id"`
	ArenaID                string    `db:"arena_id" json:"arena_id"`
	EventID                string    `db:"event_id" json:"event_id"`
	EventTitle             string    `db:"event_title" json:"event_title"`
	OwnerID                string    `db:"owner_id" json:"owner_id"`
	CategoryID             *string   `db:"category_id" json:"category_id,omitempty"`
	CategoryCode           *string   `db:"category_code" json:"category_code,omitempty"`
	CategoryName           *string   `db:"category_name" json:"category_name,omitempty"`
	BirdSpeciesID          string    `db:"bird_species_id" json:"bird_species_id"`
	BirdSpeciesName        string    `db:"bird_species_name" json:"bird_species_name"`
	RoundNumber            int       `db:"round_number" json:"round_number"`
	Title                  string    `db:"title" json:"title"`
	Status                 string    `db:"status" json:"status"`
	CreatedAt              time.Time `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time `db:"updated_at" json:"updated_at"`
	RegistrationCount      int64     `db:"registration_count" json:"registration_count"`
	ActiveParticipantCount int64     `db:"active_participant_count" json:"active_participant_count"`
	DisqualifiedCount      int64     `db:"disqualified_count" json:"disqualified_count"`
	StartTime              *string   `db:"start_time" json:"start_time,omitempty"`
	EndTime                *string   `db:"end_time" json:"end_time,omitempty"`
	CoordinatorName        *string   `db:"coordinator_name" json:"coordinator_name,omitempty"`
	HeadJudgeName          *string   `db:"head_judge_name" json:"head_judge_name,omitempty"`
	FieldJudge1            *string   `db:"field_judge_1" json:"field_judge_1,omitempty"`
	FieldJudge2            *string   `db:"field_judge_2" json:"field_judge_2,omitempty"`
	FieldJudge3            *string   `db:"field_judge_3" json:"field_judge_3,omitempty"`
	FieldJudge4            *string   `db:"field_judge_4" json:"field_judge_4,omitempty"`
	MonitorJudge1          *string   `db:"monitor_judge_1" json:"monitor_judge_1,omitempty"`
	MonitorJudge2          *string   `db:"monitor_judge_2" json:"monitor_judge_2,omitempty"`
	MonitorJudge3          *string   `db:"monitor_judge_3" json:"monitor_judge_3,omitempty"`
	MonitorJudge4          *string   `db:"monitor_judge_4" json:"monitor_judge_4,omitempty"`
}

// RegistrationView represents a row projected from cloud_read.v_registrations.
type RegistrationView struct {
	ID                     string     `db:"id" json:"id"`
	UserID                 string     `db:"user_id" json:"user_id"`
	RegistrantName         string     `db:"registrant_name" json:"registrant_name"`
	RegistrantPhone        *string    `db:"registrant_phone" json:"registrant_phone"`
	ArenaID                string     `db:"arena_id" json:"arena_id"`
	RoundID                string     `db:"round_id" json:"round_id"`
	EventID                string     `db:"event_id" json:"event_id"`
	RoundNumber            int        `db:"round_number" json:"round_number"`
	RoundTitle             string     `db:"round_title" json:"round_title"`
	BirdID                 string     `db:"bird_id" json:"bird_id"`
	BirdCode               string     `db:"bird_code" json:"bird_code"`
	BirdName               string     `db:"bird_name" json:"bird_name"`
	OwnerID                string     `db:"owner_id" json:"owner_id"`
	BirdSpeciesID          string     `db:"bird_species_id" json:"bird_species_id"`
	GantanganID            string     `db:"gantangan_id" json:"gantangan_id"`
	GantanganNumber        int        `db:"gantangan_number" json:"gantangan_number"`
	Baris                  int        `db:"baris" json:"baris"`
	Kolom                  int        `db:"kolom" json:"kolom"`
	EntryFee               string     `db:"entry_fee" json:"entry_fee"`
	PaymentMethod          string     `db:"payment_method" json:"payment_method"`
	PaymentStatus          string     `db:"payment_status" json:"payment_status"`
	HasPaymentProof        bool       `db:"has_payment_proof" json:"has_payment_proof"`
	PaymentProofMime       *string    `db:"payment_proof_mime" json:"payment_proof_mime,omitempty"`
	VerifiedAt             *time.Time `db:"verified_at" json:"verified_at,omitempty"`
	Status                 string     `db:"status" json:"status"`
	ParticipantStatus      string     `db:"participant_status" json:"participant_status"`
	DisqualificationReason *string    `db:"disqualification_reason" json:"disqualification_reason,omitempty"`
	DisqualifiedAt         *time.Time `db:"disqualified_at" json:"disqualified_at,omitempty"`
	CreatedAt              time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updated_at"`
	RegistrationNumber     *string    `db:"registration_number" json:"registration_number,omitempty"`
	RegistrationDate       *time.Time `db:"registration_date" json:"registration_date,omitempty"`
	ParticipantName        *string    `db:"participant_name" json:"participant_name,omitempty"`
	ParticipantPhone       *string    `db:"participant_phone" json:"participant_phone,omitempty"`
	OwnerName              *string    `db:"owner_name" json:"owner_name,omitempty"`
	OwnerPhone             *string    `db:"owner_phone" json:"owner_phone,omitempty"`
}

// RoundJudgeView represents an assigned judge in a round.
type RoundJudgeView struct {
	ID          string `db:"id" json:"id"`
	ArenaID     string `db:"arena_id" json:"arena_id"`
	RoundID     string `db:"round_id" json:"round_id"`
	JudgeUserID string `db:"judge_user_id" json:"judge_user_id"`
	JudgeName   string `db:"judge_name" json:"judge_name"`
}

// ScoreView represents an individual criterion score.
type ScoreView struct {
	ID             string    `db:"id" json:"id"`
	ArenaID        string    `db:"arena_id" json:"arena_id"`
	RoundID        string    `db:"round_id" json:"round_id"`
	RegistrationID string    `db:"registration_id" json:"registration_id"`
	UserID         string    `db:"user_id" json:"user_id"`
	JudgeName      string    `db:"judge_name" json:"judge_name"`
	CriteriaID     string    `db:"criteria_id" json:"criteria_id"`
	CriteriaName   string    `db:"criteria_name" json:"criteria_name"`
	ScoreValue     float64   `db:"score_value" json:"score_value"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}

// ScoreSummaryView represents the score summary of a participant.
type ScoreSummaryView struct {
	RegistrationID     string   `db:"registration_id" json:"registration_id"`
	ArenaID            string   `db:"arena_id" json:"arena_id"`
	EventID            string   `db:"event_id" json:"event_id"`
	RoundID            string   `db:"round_id" json:"round_id"`
	BirdID             string   `db:"bird_id" json:"bird_id"`
	GantanganID        string   `db:"gantangan_id" json:"gantangan_id"`
	ScoreCount         int64    `db:"score_count" json:"score_count"`
	ScoringJudgeCount  int64    `db:"scoring_judge_count" json:"scoring_judge_count"`
	ExpectedScoreCount int64    `db:"expected_score_count" json:"expected_score_count"`
	JudgeCount         int64    `db:"judge_count" json:"judge_count"`
	CriteriaCount      int64    `db:"criteria_count" json:"criteria_count"`
	TotalScore         *float64 `db:"total_score" json:"total_score"`
	AverageScore       *float64 `db:"average_score" json:"average_score"`
	IsComplete         bool     `db:"is_complete" json:"is_complete"`
}

// Input DTOs
type CreateEventRequest struct {
	OwnerID         string  `json:"owner_id"`
	Title           string  `json:"title"`
	Description     *string `json:"description"`
	EventDate       string  `json:"event_date"`
	Status          string  `json:"status"`
	Location        *string `json:"location"`
	GrandPrize      *string `json:"grand_prize"`
	PicName         *string `json:"pic_name"`
	MinParticipants int     `json:"min_participants"`
	MaxParticipants int     `json:"max_participants"`
}

type CreateRoundRequest struct {
	EventID         string  `json:"event_id"`
	CategoryID      *string `json:"category_id"`
	BirdSpeciesID   string  `json:"bird_species_id"`
	RoundNumber     int     `json:"round_number"`
	Title           string  `json:"title"`
	StartTime       *string `json:"start_time"`
	EndTime         *string `json:"end_time"`
	CoordinatorName *string `json:"coordinator_name"`
	HeadJudgeName   *string `json:"head_judge_name"`
	FieldJudge1     *string `json:"field_judge_1"`
	FieldJudge2     *string `json:"field_judge_2"`
	FieldJudge3     *string `json:"field_judge_3"`
	FieldJudge4     *string `json:"field_judge_4"`
	MonitorJudge1   *string `json:"monitor_judge_1"`
	MonitorJudge2   *string `json:"monitor_judge_2"`
	MonitorJudge3   *string `json:"monitor_judge_3"`
	MonitorJudge4   *string `json:"monitor_judge_4"`
}

type CreateRegistrationRequest struct {
	UserID             string  `json:"user_id"`
	RoundID            string  `json:"round_id"`
	BirdID             string  `json:"bird_id"`
	GantanganID        string  `json:"gantangan_id"`
	EntryFee           string  `json:"entry_fee"`
	PaymentMethod      string  `json:"payment_method"`
	RegistrationNumber *string `json:"registration_number"`
	RegistrationDate   *string `json:"registration_date"`
	ParticipantName    *string `json:"participant_name"`
	ParticipantPhone   *string `json:"participant_phone"`
	OwnerName          *string `json:"owner_name"`
	OwnerPhone         *string `json:"owner_phone"`
}

type ScoreItemRequest struct {
	CriteriaID string  `json:"criteria_id"`
	ScoreValue float64 `json:"score_value"`
}
