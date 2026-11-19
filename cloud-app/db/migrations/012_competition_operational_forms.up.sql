-- Migration 012: Add operational fields for 4 competition forms
-- Form 1: Input Data Personal (iam.users)
-- Form 2: Input Data Event (competition.events)
-- Form 3: Input Sesi & Penugasan Juri (competition.rounds)
-- Form 4: Input Data Peserta & Registrasi (competition.registrations)

-- 1. iam.users
ALTER TABLE iam.users
  ADD COLUMN IF NOT EXISTS nickname varchar(100),
  ADD COLUMN IF NOT EXISTS address text,
  ADD COLUMN IF NOT EXISTS position varchar(100);

-- 2. competition.events
ALTER TABLE competition.events
  ADD COLUMN IF NOT EXISTS location varchar(255),
  ADD COLUMN IF NOT EXISTS grand_prize varchar(255),
  ADD COLUMN IF NOT EXISTS pic_name varchar(150),
  ADD COLUMN IF NOT EXISTS min_participants integer DEFAULT 0,
  ADD COLUMN IF NOT EXISTS max_participants integer DEFAULT 0;

-- 3. competition.rounds
ALTER TABLE competition.rounds
  ADD COLUMN IF NOT EXISTS start_time varchar(20),
  ADD COLUMN IF NOT EXISTS end_time varchar(20),
  ADD COLUMN IF NOT EXISTS coordinator_name varchar(150),
  ADD COLUMN IF NOT EXISTS head_judge_name varchar(150),
  ADD COLUMN IF NOT EXISTS field_judge_1 varchar(150),
  ADD COLUMN IF NOT EXISTS field_judge_2 varchar(150),
  ADD COLUMN IF NOT EXISTS field_judge_3 varchar(150),
  ADD COLUMN IF NOT EXISTS field_judge_4 varchar(150),
  ADD COLUMN IF NOT EXISTS monitor_judge_1 varchar(150),
  ADD COLUMN IF NOT EXISTS monitor_judge_2 varchar(150),
  ADD COLUMN IF NOT EXISTS monitor_judge_3 varchar(150),
  ADD COLUMN IF NOT EXISTS monitor_judge_4 varchar(150);

-- 4. competition.registrations
ALTER TABLE competition.registrations
  ADD COLUMN IF NOT EXISTS registration_number varchar(100),
  ADD COLUMN IF NOT EXISTS registration_date date DEFAULT CURRENT_DATE,
  ADD COLUMN IF NOT EXISTS participant_name varchar(150),
  ADD COLUMN IF NOT EXISTS participant_phone varchar(30),
  ADD COLUMN IF NOT EXISTS owner_name varchar(150),
  ADD COLUMN IF NOT EXISTS owner_phone varchar(30);

-- Update cloud_read views with the new operational columns
CREATE OR REPLACE VIEW cloud_read.v_users AS
SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
       password_hash IS NOT NULL AS has_password, created_at, updated_at,
       version, deleted_at, nickname, address, position
FROM iam.users;

CREATE OR REPLACE VIEW cloud_read.v_events AS
WITH counts AS (SELECT event_id, count(*) AS round_count FROM competition.rounds GROUP BY event_id)
SELECT e.id, e.arena_id, a.name AS arena_name, e.owner_id, u.name AS organizer_name,
       e.title, e.description, e.event_date, e.status, e.created_at, e.updated_at,
       coalesce(c.round_count, 0) AS round_count,
       e.location, e.grand_prize, e.pic_name, e.min_participants, e.max_participants
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
       coalesce(n.disqualified_count, 0) AS disqualified_count,
       r.start_time, r.end_time, r.coordinator_name, r.head_judge_name,
       r.field_judge_1, r.field_judge_2, r.field_judge_3, r.field_judge_4,
       r.monitor_judge_1, r.monitor_judge_2, r.monitor_judge_3, r.monitor_judge_4
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
       t.disqualified_at, t.created_at, t.updated_at,
       t.registration_number, t.registration_date, t.participant_name, t.participant_phone,
       t.owner_name, t.owner_phone
FROM competition.registrations t JOIN iam.users u ON u.id = t.user_id
JOIN competition.rounds r ON r.id = t.round_id JOIN asset.birds b ON b.id = t.bird_id
JOIN asset.gantangan g ON g.id = t.gantangan_id;

GRANT SELECT ON cloud_read.v_users, cloud_read.v_events, cloud_read.v_rounds, cloud_read.v_registrations TO kicau_cloud_app, kicau_cloud_admin_reader;
