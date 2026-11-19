-- Revert Migration 012: Drop added columns and revert views

CREATE OR REPLACE VIEW cloud_read.v_users AS
SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
       password_hash IS NOT NULL AS has_password, created_at, updated_at,
       version, deleted_at
FROM iam.users;

CREATE OR REPLACE VIEW cloud_read.v_events AS
WITH counts AS (SELECT event_id, count(*) AS round_count FROM competition.rounds GROUP BY event_id)
SELECT e.id, e.arena_id, a.name AS arena_name, e.owner_id, u.name AS organizer_name,
       e.title, e.description, e.event_date, e.status, e.created_at, e.updated_at,
       coalesce(c.round_count, 0) AS round_count
FROM competition.events e JOIN asset.arenas a ON a.id = e.arena_id
JOIN iam.users u ON u.id = e.owner_id LEFT JOIN counts c ON c.event_id = e.id;

CREATE OR REPLACE VIEW cloud_read.v_rounds AS
WITH counts AS (
    SELECT round_id, count(*) AS registration_count,
           count(*) FILTER (WHERE status = 'CONFIRMED' AND payment_status = 'VERIFIED' AND participant_status = 'ACTIVE') AS active_participant_count,
           count(*) FILTER (WHERE participant_status = 'DISQUALIFIED') AS disqualified_count
    FROM competition.registrations GROUP BY round_id
)
SELECT r.id, r.arena_id, r.event_id, e.title AS event_title, e.owner_id,
       r.category_id, c.code AS category_code, c.name AS category_name,
       r.bird_species_id, s.name AS bird_species_name, r.round_number, r.title,
       r.status, r.created_at, r.updated_at, coalesce(n.registration_count, 0) AS registration_count,
       coalesce(n.active_participant_count, 0) AS active_participant_count,
       coalesce(n.disqualified_count, 0) AS disqualified_count
FROM competition.rounds r JOIN competition.events e ON e.id = r.event_id
JOIN asset.bird_species s ON s.id = r.bird_species_id
LEFT JOIN competition.categories c ON c.id = r.category_id
LEFT JOIN counts n ON n.round_id = r.id;

CREATE OR REPLACE VIEW cloud_read.v_registrations AS
SELECT t.id, t.user_id, u.name AS registrant_name, u.phone AS registrant_phone,
       t.arena_id, t.round_id, r.event_id, r.round_number, r.title AS round_title,
       t.bird_id, b.bird_code, b.name AS bird_name, b.owner_id, b.bird_species_id,
       t.gantangan_id, g.number AS gantangan_number, g.baris, g.kolom,
       t.entry_fee, t.payment_method, t.payment_status,
       t.payment_proof IS NOT NULL AS has_payment_proof, t.payment_proof_mime,
       t.verified_at, t.status, t.participant_status, t.disqualification_reason,
       t.disqualified_at, t.created_at, t.updated_at
FROM competition.registrations t JOIN iam.users u ON u.id = t.user_id
JOIN competition.rounds r ON r.id = t.round_id JOIN asset.birds b ON b.id = t.bird_id
JOIN asset.gantangan g ON g.id = t.gantangan_id;

ALTER TABLE competition.registrations
  DROP COLUMN IF EXISTS registration_number,
  DROP COLUMN IF EXISTS registration_date,
  DROP COLUMN IF EXISTS participant_name,
  DROP COLUMN IF EXISTS participant_phone,
  DROP COLUMN IF EXISTS owner_name,
  DROP COLUMN IF EXISTS owner_phone;

ALTER TABLE competition.rounds
  DROP COLUMN IF EXISTS start_time,
  DROP COLUMN IF EXISTS end_time,
  DROP COLUMN IF EXISTS coordinator_name,
  DROP COLUMN IF EXISTS head_judge_name,
  DROP COLUMN IF EXISTS field_judge_1,
  DROP COLUMN IF EXISTS field_judge_2,
  DROP COLUMN IF EXISTS field_judge_3,
  DROP COLUMN IF EXISTS field_judge_4,
  DROP COLUMN IF EXISTS monitor_judge_1,
  DROP COLUMN IF EXISTS monitor_judge_2,
  DROP COLUMN IF EXISTS monitor_judge_3,
  DROP COLUMN IF EXISTS monitor_judge_4;

ALTER TABLE competition.events
  DROP COLUMN IF EXISTS location,
  DROP COLUMN IF EXISTS grand_prize,
  DROP COLUMN IF EXISTS pic_name,
  DROP COLUMN IF EXISTS min_participants,
  DROP COLUMN IF EXISTS max_participants;

ALTER TABLE iam.users
  DROP COLUMN IF EXISTS nickname,
  DROP COLUMN IF EXISTS address,
  DROP COLUMN IF EXISTS position;
