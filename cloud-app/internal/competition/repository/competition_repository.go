package repository

import (
	"context"

	"kicau.cloud/control/internal/competition/contract"
	"kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/infra/postgres"
)

// ---------------- Events ----------------

type eventRepoImpl struct{ store *postgres.Store }

func NewEventRepository(s *postgres.Store) contract.EventRepository {
	return &eventRepoImpl{store: s}
}

func (r *eventRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.EventView, error) {
	query := `
		SELECT id, arena_id, arena_name, owner_id, organizer_name, title, description,
		       event_date, status, created_at, updated_at, round_count, location, grand_prize,
		       pic_name, min_participants, max_participants
		FROM cloud_read.v_events
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
	`
	return postgres.QueryOne[model.EventView](ctx, r.store, query, arenaID, id)
}

func (r *eventRepoImpl) List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.EventView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, arena_id, arena_name, owner_id, organizer_name, title, description,
		       event_date, status, created_at, updated_at, round_count, location, grand_prize,
		       pic_name, min_participants, max_participants
		FROM cloud_read.v_events
		WHERE arena_id = $1::text::bigint AND ($2::text::bigint IS NULL OR id > $2::text::bigint)
		ORDER BY id
		LIMIT $3
	`
	return postgres.QueryMany[model.EventView](ctx, r.store, query, arenaID, cursor, limit)
}

func (r *eventRepoImpl) Create(ctx context.Context, arenaID string, req *model.CreateEventRequest) (*model.EventView, error) {
	query := `
		INSERT INTO competition.events(arena_id, owner_id, title, description, event_date, status, location, grand_prize, pic_name, min_participants, max_participants)
		VALUES($1::text::bigint, $2::text::bigint, $3, $4, $5::date, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`
	row, err := r.store.QueryRow(ctx, query, arenaID, req.OwnerID, req.Title, req.Description, req.EventDate, req.Status, req.Location, req.GrandPrize, req.PicName, req.MinParticipants, req.MaxParticipants)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *eventRepoImpl) Update(ctx context.Context, arenaID, id string, req *model.CreateEventRequest) (*model.EventView, error) {
	query := `
		UPDATE competition.events SET
			owner_id = coalesce($3::text::bigint, owner_id),
			title = coalesce($4, title),
			description = coalesce($5, description),
			event_date = coalesce($6::date, event_date),
			status = coalesce($7, status),
			location = coalesce($8, location),
			grand_prize = coalesce($9, grand_prize),
			pic_name = coalesce($10, pic_name),
			min_participants = coalesce($11, min_participants),
			max_participants = coalesce($12, max_participants),
			updated_at = CURRENT_TIMESTAMP
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
	`
	err := r.store.ExecSQL(ctx, query, arenaID, id, req.OwnerID, req.Title, req.Description, req.EventDate, req.Status, req.Location, req.GrandPrize, req.PicName, req.MinParticipants, req.MaxParticipants)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *eventRepoImpl) Delete(ctx context.Context, arenaID, id string) error {
	query := `DELETE FROM competition.events WHERE arena_id = $1::text::bigint AND id = $2::text::bigint`
	return r.store.ExecSQL(ctx, query, arenaID, id)
}

// ---------------- Rounds ----------------

type roundRepoImpl struct{ store *postgres.Store }

func NewRoundRepository(s *postgres.Store) contract.RoundRepository {
	return &roundRepoImpl{store: s}
}

func (r *roundRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.RoundView, error) {
	query := `
		SELECT id, arena_id, event_id, event_title, owner_id, category_id, category_code, category_name,
		       bird_species_id, bird_species_name, round_number, title, status, created_at, updated_at,
		       registration_count, active_participant_count, disqualified_count,
		       start_time, end_time, coordinator_name, head_judge_name,
		       field_judge_1, field_judge_2, field_judge_3, field_judge_4,
		       monitor_judge_1, monitor_judge_2, monitor_judge_3, monitor_judge_4
		FROM cloud_read.v_rounds
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
	`
	return postgres.QueryOne[model.RoundView](ctx, r.store, query, arenaID, id)
}

func (r *roundRepoImpl) List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.RoundView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, arena_id, event_id, event_title, owner_id, category_id, category_code, category_name,
		       bird_species_id, bird_species_name, round_number, title, status, created_at, updated_at,
		       registration_count, active_participant_count, disqualified_count,
		       start_time, end_time, coordinator_name, head_judge_name,
		       field_judge_1, field_judge_2, field_judge_3, field_judge_4,
		       monitor_judge_1, monitor_judge_2, monitor_judge_3, monitor_judge_4
		FROM cloud_read.v_rounds
		WHERE arena_id = $1::text::bigint AND ($2::text::bigint IS NULL OR id > $2::text::bigint)
		ORDER BY id
		LIMIT $3
	`
	return postgres.QueryMany[model.RoundView](ctx, r.store, query, arenaID, cursor, limit)
}

func (r *roundRepoImpl) Create(ctx context.Context, arenaID string, req *model.CreateRoundRequest) (*model.RoundView, error) {
	query := `
		INSERT INTO competition.rounds(arena_id, event_id, category_id, bird_species_id, round_number, title,
		                               start_time, end_time, coordinator_name, head_judge_name,
		                               field_judge_1, field_judge_2, field_judge_3, field_judge_4,
		                               monitor_judge_1, monitor_judge_2, monitor_judge_3, monitor_judge_4)
		VALUES($1::text::bigint, $2::text::bigint, $3::text::bigint, $4::text::bigint, $5, $6,
		       $7, $8, $9, $10,
		       $11, $12, $13, $14,
		       $15, $16, $17, $18)
		RETURNING id
	`
	row, err := r.store.QueryRow(ctx, query, arenaID, req.EventID, req.CategoryID, req.BirdSpeciesID, req.RoundNumber, req.Title, req.StartTime, req.EndTime, req.CoordinatorName, req.HeadJudgeName, req.FieldJudge1, req.FieldJudge2, req.FieldJudge3, req.FieldJudge4, req.MonitorJudge1, req.MonitorJudge2, req.MonitorJudge3, req.MonitorJudge4)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *roundRepoImpl) Update(ctx context.Context, arenaID, id string, req *model.CreateRoundRequest) (*model.RoundView, error) {
	query := `
		UPDATE competition.rounds SET
			event_id = coalesce($3::text::bigint, event_id),
			category_id = coalesce($4::text::bigint, category_id),
			bird_species_id = coalesce($5::text::bigint, bird_species_id),
			round_number = coalesce($6, round_number),
			title = coalesce($7, title),
			start_time = coalesce($8, start_time),
			end_time = coalesce($9, end_time),
			coordinator_name = coalesce($10, coordinator_name),
			head_judge_name = coalesce($11, head_judge_name),
			field_judge_1 = coalesce($12, field_judge_1),
			field_judge_2 = coalesce($13, field_judge_2),
			field_judge_3 = coalesce($14, field_judge_3),
			field_judge_4 = coalesce($15, field_judge_4),
			monitor_judge_1 = coalesce($16, monitor_judge_1),
			monitor_judge_2 = coalesce($17, monitor_judge_2),
			monitor_judge_3 = coalesce($18, monitor_judge_3),
			monitor_judge_4 = coalesce($19, monitor_judge_4),
			updated_at = CURRENT_TIMESTAMP
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
	`
	err := r.store.ExecSQL(ctx, query, arenaID, id, req.EventID, req.CategoryID, req.BirdSpeciesID, req.RoundNumber, req.Title, req.StartTime, req.EndTime, req.CoordinatorName, req.HeadJudgeName, req.FieldJudge1, req.FieldJudge2, req.FieldJudge3, req.FieldJudge4, req.MonitorJudge1, req.MonitorJudge2, req.MonitorJudge3, req.MonitorJudge4)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *roundRepoImpl) Delete(ctx context.Context, arenaID, id string) error {
	query := `DELETE FROM competition.rounds WHERE arena_id = $1::text::bigint AND id = $2::text::bigint`
	return r.store.ExecSQL(ctx, query, arenaID, id)
}

func (r *roundRepoImpl) Lock(ctx context.Context, arenaID, id string) (*model.RoundView, error) {
	query := `
		SELECT status
		FROM competition.rounds
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint FOR UPDATE
	`
	return postgres.QueryOne[model.RoundView](ctx, r.store, query, arenaID, id)
}

func (r *roundRepoImpl) SetStatus(ctx context.Context, arenaID, id, expectedStatus, newStatus string) (*model.RoundView, error) {
	query := `UPDATE competition.rounds SET status = $4, updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status = $3`
	if err := r.store.ExecSQL(ctx, query, arenaID, id, expectedStatus, newStatus); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

// ---------------- Registrations ----------------

type registrationRepoImpl struct{ store *postgres.Store }

func NewRegistrationRepository(s *postgres.Store) contract.RegistrationRepository {
	return &registrationRepoImpl{store: s}
}

func (r *registrationRepoImpl) GetByID(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `
		SELECT id, user_id, registrant_name, registrant_phone, arena_id, round_id, event_id,
		       round_number, round_title, bird_id, bird_code, bird_name, owner_id, bird_species_id,
		       gantangan_id, gantangan_number, baris, kolom, entry_fee, payment_method, payment_status,
		       has_payment_proof, payment_proof_mime, verified_at, status, participant_status,
		       disqualification_reason, disqualified_at, created_at, updated_at,
		       registration_number, registration_date, participant_name, participant_phone,
		       owner_name, owner_phone
		FROM cloud_read.v_registrations
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
	`
	return postgres.QueryOne[model.RegistrationView](ctx, r.store, query, arenaID, id)
}

func (r *registrationRepoImpl) List(ctx context.Context, arenaID string, roundID, cursor *string, limit int) ([]model.RegistrationView, error) {
	if limit < 1 || limit > 100 {
		limit = 25
	}
	query := `
		SELECT id, user_id, registrant_name, registrant_phone, arena_id, round_id, event_id,
		       round_number, round_title, bird_id, bird_code, bird_name, owner_id, bird_species_id,
		       gantangan_id, gantangan_number, baris, kolom, entry_fee, payment_method, payment_status,
		       has_payment_proof, payment_proof_mime, verified_at, status, participant_status,
		       disqualification_reason, disqualified_at, created_at, updated_at,
		       registration_number, registration_date, participant_name, participant_phone,
		       owner_name, owner_phone
		FROM cloud_read.v_registrations
		WHERE arena_id = $1::text::bigint
		  AND ($2::text::bigint IS NULL OR round_id = $2::text::bigint)
		  AND ($3::text::bigint IS NULL OR id > $3::text::bigint)
		ORDER BY id
		LIMIT $4
	`
	return postgres.QueryMany[model.RegistrationView](ctx, r.store, query, arenaID, roundID, cursor, limit)
}

func (r *registrationRepoImpl) Create(ctx context.Context, arenaID string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error) {
	query := `
		INSERT INTO competition.registrations(user_id, arena_id, round_id, bird_id, gantangan_id, entry_fee, payment_method,
		                                      registration_number, registration_date, participant_name, participant_phone, owner_name, owner_phone)
		SELECT u.id, $2::text::bigint, r.id, b.id, NULLIF($5::text, '')::bigint, $6::numeric, NULLIF($7, ''),
		       $8, coalesce($9::date, CURRENT_DATE), $10, $11, $12, $13
		FROM iam.users u
		CROSS JOIN asset.birds b
		JOIN competition.rounds r ON r.bird_species_id = b.bird_species_id
		WHERE u.id = $1::text::bigint AND u.deleted_at IS NULL AND u.is_active
		  AND b.id = $4::text::bigint AND b.deleted_at IS NULL
		  AND r.id = $3::text::bigint AND r.arena_id = $2::text::bigint
		RETURNING id
	`
	row, err := r.store.QueryRow(ctx, query, req.UserID, arenaID, req.RoundID, req.BirdID, req.GantanganID, req.EntryFee, req.PaymentMethod, req.RegistrationNumber, req.RegistrationDate, req.ParticipantName, req.ParticipantPhone, req.OwnerName, req.OwnerPhone)
	if err != nil {
		return nil, err
	}
	var id string
	if err := row.Scan(&id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) Update(ctx context.Context, arenaID, id string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error) {
	query := `
		WITH updated AS (
			UPDATE competition.registrations SET
				user_id = $3::text::bigint,
				round_id = $4::text::bigint,
				bird_id = $5::text::bigint,
				gantangan_id = CASE WHEN $6::text IS NULL THEN gantangan_id ELSE NULLIF($6, '')::bigint END,
				entry_fee = $7::numeric,
				payment_method = COALESCE(NULLIF($8, ''), payment_method),
				registration_number = COALESCE(NULLIF($9, ''), registration_number),
				registration_date = COALESCE($10::date, registration_date, CURRENT_DATE),
				participant_name = $11,
				participant_phone = $12,
				owner_name = $13,
				owner_phone = $14,
				updated_at = CURRENT_TIMESTAMP
			WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status = 'PENDING_PAYMENT' AND payment_status <> 'VERIFIED'
			RETURNING id
		)
		UPDATE competition.registrations SET
			gantangan_id = NULLIF($6, '')::bigint,
			updated_at = CURRENT_TIMESTAMP
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint
		  AND status = 'CONFIRMED' AND gantangan_id IS NULL
		  AND NULLIF($6, '') IS NOT NULL
		  AND NOT EXISTS (SELECT 1 FROM updated)
	`
	err := r.store.ExecSQL(ctx, query, arenaID, id, req.UserID, req.RoundID, req.BirdID, req.GantanganID, req.EntryFee, req.PaymentMethod, req.RegistrationNumber, req.RegistrationDate, req.ParticipantName, req.ParticipantPhone, req.OwnerName, req.OwnerPhone)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) Lock(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `
		SELECT id, user_id, registrant_name, registrant_phone, arena_id, round_id, event_id,
		       round_number, round_title, bird_id, bird_code, bird_name, owner_id, bird_species_id,
		       gantangan_id, gantangan_number, baris, kolom, entry_fee, payment_method, payment_status,
		       has_payment_proof, payment_proof_mime, verified_at, status, participant_status,
		       disqualification_reason, disqualified_at, created_at, updated_at,
		       registration_number, registration_date, participant_name, participant_phone,
		       owner_name, owner_phone
		FROM competition.registrations
		WHERE arena_id = $1::text::bigint AND id = $2::text::bigint FOR UPDATE
	`
	return postgres.QueryOne[model.RegistrationView](ctx, r.store, query, arenaID, id)
}

func (r *registrationRepoImpl) VerifyPayment(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `UPDATE competition.registrations SET payment_status = 'VERIFIED', verified_at = CURRENT_TIMESTAMP, status = 'CONFIRMED', updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status = 'PENDING_PAYMENT'`
	if err := r.store.ExecSQL(ctx, query, arenaID, id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) RejectPayment(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `UPDATE competition.registrations SET payment_status = 'REJECTED', verified_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status = 'PENDING_PAYMENT'`
	if err := r.store.ExecSQL(ctx, query, arenaID, id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) Cancel(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `UPDATE competition.registrations SET status = 'CANCELLED', updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status <> 'CANCELLED'`
	if err := r.store.ExecSQL(ctx, query, arenaID, id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) Disqualify(ctx context.Context, arenaID, id, reason string) (*model.RegistrationView, error) {
	query := `UPDATE competition.registrations SET participant_status = 'DISQUALIFIED', disqualification_reason = $3, disqualified_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND participant_status = 'ACTIVE'`
	if err := r.store.ExecSQL(ctx, query, arenaID, id, reason); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) Requalify(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	query := `UPDATE competition.registrations SET participant_status = 'ACTIVE', disqualification_reason = NULL, disqualified_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND participant_status = 'DISQUALIFIED'`
	if err := r.store.ExecSQL(ctx, query, arenaID, id); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, arenaID, id)
}

func (r *registrationRepoImpl) SetProof(ctx context.Context, arenaID, id string, imageBytes []byte, mimeType string) error {
	query := `UPDATE competition.registrations SET payment_proof = $3::bytea, payment_proof_mime = $4, updated_at = CURRENT_TIMESTAMP WHERE arena_id = $1::text::bigint AND id = $2::text::bigint AND status = 'PENDING_PAYMENT' AND payment_status <> 'VERIFIED'`
	return r.store.ExecSQL(ctx, query, arenaID, id, imageBytes, mimeType)
}

func (r *registrationRepoImpl) ReadProof(ctx context.Context, arenaID, id string) ([]byte, string, error) {
	query := `SELECT payment_proof, payment_proof_mime FROM competition.registrations WHERE arena_id = $1::text::bigint AND id = $2::text::bigint`
	row, err := r.store.QueryRow(ctx, query, arenaID, id)
	if err != nil {
		return nil, "", err
	}
	var b []byte
	var m *string
	if err := row.Scan(&b, &m); err != nil {
		return nil, "", err
	}
	mime := "image/jpeg"
	if m != nil && *m != "" {
		mime = *m
	}
	return b, mime, nil
}

func (r *registrationRepoImpl) RoundParticipants(ctx context.Context, arenaID, roundID string) ([]model.RegistrationView, error) {
	query := `
		SELECT id, user_id, registrant_name, registrant_phone, arena_id, round_id, event_id,
		       round_number, round_title, bird_id, bird_code, bird_name, owner_id, bird_species_id,
		       gantangan_id, gantangan_number, baris, kolom, entry_fee, payment_method, payment_status,
		       has_payment_proof, payment_proof_mime, verified_at, status, participant_status,
		       disqualification_reason, disqualified_at, created_at, updated_at,
		       registration_number, registration_date, participant_name, participant_phone,
		       owner_name, owner_phone
		FROM cloud_read.v_registrations
		WHERE arena_id = $1::text::bigint AND round_id = $2::text::bigint AND status = 'CONFIRMED' AND payment_status = 'VERIFIED'
		ORDER BY gantangan_number, id
	`
	return postgres.QueryMany[model.RegistrationView](ctx, r.store, query, arenaID, roundID)
}

// ---------------- Judging ----------------

type judgingRepoImpl struct{ store *postgres.Store }

func NewJudgingRepository(s *postgres.Store) contract.JudgingRepository {
	return &judgingRepoImpl{store: s}
}

func (r *judgingRepoImpl) RoundJudges(ctx context.Context, arenaID, roundID string) ([]model.RoundJudgeView, error) {
	query := `SELECT id, arena_id, round_id, judge_user_id, judge_name FROM cloud_read.v_round_judges WHERE arena_id = $1::text::bigint AND round_id = $2::text::bigint`
	return postgres.QueryMany[model.RoundJudgeView](ctx, r.store, query, arenaID, roundID)
}

func (r *judgingRepoImpl) AssignJudge(ctx context.Context, arenaID, roundID, judgeUserID string) error {
	query := `INSERT INTO judging.round_judges(round_id, judge_user_id) VALUES($1::text::bigint, $2::text::bigint) ON CONFLICT DO NOTHING`
	return r.store.ExecSQL(ctx, query, roundID, judgeUserID)
}

func (r *judgingRepoImpl) RemoveJudge(ctx context.Context, arenaID, roundID, judgeUserID string) error {
	query := `DELETE FROM judging.round_judges WHERE round_id = $1::text::bigint AND judge_user_id = $2::text::bigint`
	return r.store.ExecSQL(ctx, query, roundID, judgeUserID)
}

func (r *judgingRepoImpl) SaveScore(ctx context.Context, arenaID, roundID, registrationID, judgeUserID, criteriaID string, scoreValue float64) error {
	query := `
		INSERT INTO judging.round_scores(arena_id, round_id, registration_id, user_id, criteria_id, score_value)
		VALUES($1::text::bigint, $2::text::bigint, $3::text::bigint, $4::text::bigint, $5::text::bigint, $6)
		ON CONFLICT (registration_id, user_id, criteria_id)
		DO UPDATE SET score_value = EXCLUDED.score_value, updated_at = CURRENT_TIMESTAMP
	`
	return r.store.ExecSQL(ctx, query, arenaID, roundID, registrationID, judgeUserID, criteriaID, scoreValue)
}

func (r *judgingRepoImpl) ReadScores(ctx context.Context, arenaID, roundID, registrationID string) ([]model.ScoreView, error) {
	query := `
		SELECT id, arena_id, round_id, registration_id, user_id, judge_name, criteria_id, criteria_name,
		       score_value, created_at, updated_at
		FROM cloud_read.v_round_scores
		WHERE arena_id = $1::text::bigint AND round_id = $2::text::bigint AND registration_id = $3::text::bigint
		ORDER BY id
	`
	return postgres.QueryMany[model.ScoreView](ctx, r.store, query, arenaID, roundID, registrationID)
}

func (r *judgingRepoImpl) RoundRecap(ctx context.Context, arenaID, roundID string) ([]model.ScoreSummaryView, error) {
	query := `
		SELECT registration_id, arena_id, event_id, round_id, bird_id, gantangan_id,
		       score_count, scoring_judge_count, expected_score_count, judge_count, criteria_count,
		       total_score, average_score, is_complete
		FROM cloud_read.v_round_score_summary
		WHERE arena_id = $1::text::bigint AND round_id = $2::text::bigint
		ORDER BY total_score DESC NULLS LAST, gantangan_id
	`
	return postgres.QueryMany[model.ScoreSummaryView](ctx, r.store, query, arenaID, roundID)
}
