-- PostgreSQL 18; source: full_db.dump followed by migrations 001..003.
-- Empty development database only. Execute in one transaction (psql -1).
-- Existing read-layer migrations are immutable; this migration retires them.
DO $empty$
DECLARE table_name text; occupied boolean;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['asset.bird_owners', 'asset.birds', 'asset.devices', 'asset.lapaks', 'competition.events', 'competition.round_participants', 'competition.rounds', 'iam.roles', 'iam.user_auth_providers', 'iam.user_sessions', 'iam.users', 'judging.round_scores', 'judging.score_criteria', 'media.round_recordings', 'member.plans', 'member.user_subscriptions']
    LOOP
        EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE', table_name);
        EXECUTE format('SELECT EXISTS (SELECT FROM %s)', table_name) INTO occupied;
        IF occupied THEN
            RAISE EXCEPTION 'Migration requires empty tables; % contains data. No changes applied.', table_name;
        END IF;
    END LOOP;
END;
$empty$;

DO $baseline$
BEGIN
    IF to_regclass('cloud_read.mv_round_score_summary') IS NULL THEN
        RAISE EXCEPTION 'Apply migrations 001..003 before 004';
    END IF;
END;
$baseline$;

DROP MATERIALIZED VIEW cloud_read.mv_round_score_summary;

DROP VIEW cloud_read.v_round_recordings;
DROP VIEW cloud_read.v_round_scores;
DROP VIEW cloud_read.v_score_criteria;
DROP VIEW cloud_read.v_user_subscriptions;
DROP VIEW cloud_read.v_plans;
DROP VIEW cloud_read.v_user_sessions;
DROP VIEW cloud_read.v_user_auth_providers;
DROP VIEW cloud_read.v_roles;
DROP VIEW cloud_read.v_users;
DROP VIEW cloud_read.v_devices;
DROP VIEW cloud_read.v_lapaks;
DROP VIEW cloud_read.v_bird_owners;
DROP VIEW cloud_read.v_birds;
DROP VIEW cloud_read.v_round_participants;
DROP VIEW cloud_read.v_rounds;
DROP VIEW cloud_read.v_events;
DROP SCHEMA cloud_read;

DROP TABLE media.round_recordings;
DROP TABLE judging.round_scores;
DROP TABLE judging.score_criteria;
DROP TABLE competition.round_participants;
DROP TABLE competition.rounds;
DROP TABLE competition.events;
DROP TABLE asset.devices;
DROP TABLE asset.bird_owners;
DROP TABLE asset.birds;
DROP TABLE asset.lapaks;

-- IAM auth providers and membership tables retain their existing structure.
ALTER TABLE iam.users DROP COLUMN role_id;
ALTER TABLE iam.user_sessions ADD COLUMN device_id uuid NOT NULL;
CREATE UNIQUE INDEX user_sessions_device_active_uidx ON iam.user_sessions (user_id, device_id)
    WHERE revoked_at IS NULL;
COMMENT ON COLUMN iam.user_sessions.device_id IS
    'Browser/device identity. Login UseCase serializes by user and allows at most three distinct unexpired, unrevoked devices; expired slots must be revoked/reused.';

CREATE TABLE asset.arenas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    address text,
    timezone varchar(100) NOT NULL DEFAULT 'Asia/Jakarta',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE iam.user_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE RESTRICT,
    role_id uuid NOT NULL REFERENCES iam.roles(id) ON DELETE RESTRICT,
    UNIQUE (user_id, role_id)
);
CREATE TABLE iam.arena_user_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE RESTRICT,
    role_id uuid NOT NULL REFERENCES iam.roles(id) ON DELETE RESTRICT,
    UNIQUE (arena_id, user_id, role_id)
);
CREATE INDEX arena_user_roles_user_idx ON iam.arena_user_roles (user_id, arena_id);

CREATE TABLE asset.gantangan (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    number integer NOT NULL CHECK (number > 0),
    description text,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (arena_id, number),
    UNIQUE (id, arena_id)
);
CREATE TABLE asset.engines (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    is_active boolean NOT NULL DEFAULT true,
    last_heartbeat_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, arena_id)
);
CREATE UNIQUE INDEX engines_one_active_per_arena_uidx ON asset.engines (arena_id) WHERE is_active;
CREATE TABLE asset.devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    engine_id uuid NOT NULL,
    gantangan_id uuid,
    type varchar(20) NOT NULL,
    ip_address varchar(45) NOT NULL,
    rtsp_url text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (engine_id, arena_id) REFERENCES asset.engines(id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (gantangan_id, arena_id) REFERENCES asset.gantangan(id, arena_id) ON DELETE RESTRICT
);
CREATE INDEX devices_engine_idx ON asset.devices (engine_id, arena_id);
CREATE INDEX devices_gantangan_idx ON asset.devices (gantangan_id, arena_id);
CREATE TABLE asset.owners (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid UNIQUE REFERENCES iam.users(id) ON DELETE RESTRICT,
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    phone varchar(30),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE asset.birds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES asset.owners(id) ON DELETE RESTRICT,
    ring_number varchar(50) UNIQUE,
    bird_type varchar(50) NOT NULL,
    name varchar(100) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX birds_owner_idx ON asset.birds (owner_id);
CREATE TABLE asset.bird_owner_transfers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bird_id uuid NOT NULL REFERENCES asset.birds(id) ON DELETE RESTRICT,
    from_owner_id uuid NOT NULL REFERENCES asset.owners(id) ON DELETE RESTRICT,
    to_owner_id uuid NOT NULL REFERENCES asset.owners(id) ON DELETE RESTRICT,
    performed_by_user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE RESTRICT,
    reason text NOT NULL CHECK (btrim(reason) <> ''),
    transferred_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (from_owner_id <> to_owner_id)
);
CREATE INDEX bird_owner_transfers_bird_idx ON asset.bird_owner_transfers (bird_id, transferred_at, id);

CREATE TABLE competition.events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    title varchar(150) NOT NULL,
    date date NOT NULL,
    location text NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'scheduled',
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, arena_id)
);
CREATE INDEX events_arena_date_idx ON competition.events (arena_id, date, id);
CREATE TABLE competition.categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    code varchar(50) NOT NULL CHECK (btrim(code) <> ''),
    name varchar(100) NOT NULL CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (arena_id, code),
    UNIQUE (id, arena_id)
);
CREATE TABLE competition.rounds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    event_id uuid NOT NULL,
    category_id uuid NOT NULL,
    round_number integer NOT NULL CHECK (round_number > 0),
    bird_type varchar(50) NOT NULL,
    entry_fee numeric(12,2) NOT NULL DEFAULT 0 CHECK (entry_fee >= 0),
    phase varchar(20) NOT NULL DEFAULT 'PREPARED'
        CHECK (phase IN ('PREPARED','RUNNING','JUDGING','CLOSED','INTERRUPTED','CANCELLED')),
    interrupted_from_phase varchar(20),
    session_duration_minutes integer NOT NULL DEFAULT 10 CHECK (session_duration_minutes > 0),
    judging_duration_minutes integer NOT NULL DEFAULT 15 CHECK (judging_duration_minutes > 0),
    start_time timestamptz,
    end_time timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (event_id, round_number),
    UNIQUE (id, arena_id),
    FOREIGN KEY (event_id, arena_id) REFERENCES competition.events(id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (category_id, arena_id) REFERENCES competition.categories(id, arena_id) ON DELETE RESTRICT,
    CHECK ((phase = 'INTERRUPTED' AND interrupted_from_phase IS NOT NULL
            AND interrupted_from_phase IN ('RUNNING','JUDGING'))
        OR (phase <> 'INTERRUPTED' AND interrupted_from_phase IS NULL)),
    CHECK (end_time IS NULL OR start_time IS NOT NULL AND end_time >= start_time)
);
CREATE UNIQUE INDEX rounds_one_active_per_arena_uidx ON competition.rounds (arena_id)
    WHERE phase IN ('RUNNING','JUDGING','INTERRUPTED');
CREATE INDEX rounds_category_idx ON competition.rounds (category_id, arena_id);

CREATE TABLE competition.registrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    registered_by_user_id uuid REFERENCES iam.users(id) ON DELETE RESTRICT,
    created_by_user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE RESTRICT,
    registration_source varchar(20) NOT NULL CHECK (registration_source IN ('MANUAL','ONLINE')),
    registrant_name varchar(150) NOT NULL CHECK (btrim(registrant_name) <> ''),
    registrant_phone varchar(30) NOT NULL CHECK (btrim(registrant_phone) <> ''),
    total_amount numeric(12,2) NOT NULL DEFAULT 0 CHECK (total_amount >= 0),
    status varchar(30) NOT NULL DEFAULT 'DRAFT'
        CHECK (status IN ('DRAFT','PENDING_PAYMENT','CONFIRMED','CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, arena_id),
    CHECK (registration_source <> 'ONLINE' OR registered_by_user_id IS NOT NULL)
);
CREATE INDEX registrations_arena_idx ON competition.registrations (arena_id, created_at, id);
CREATE INDEX registrations_registrant_idx ON competition.registrations (registered_by_user_id);
CREATE TABLE competition.registration_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    registration_id uuid NOT NULL,
    round_id uuid NOT NULL,
    bird_id uuid NOT NULL REFERENCES asset.birds(id) ON DELETE RESTRICT,
    gantangan_id uuid,
    entry_fee_snapshot numeric(12,2) NOT NULL CHECK (entry_fee_snapshot >= 0),
    status varchar(30) NOT NULL DEFAULT 'PENDING_PAYMENT'
        CHECK (status IN ('DRAFT','PENDING_PAYMENT','CONFIRMED','CANCELLED','DISQUALIFIED','WITHDRAWN')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, round_id, arena_id),
    UNIQUE (id, arena_id),
    FOREIGN KEY (registration_id, arena_id) REFERENCES competition.registrations(id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (round_id, arena_id) REFERENCES competition.rounds(id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (gantangan_id, arena_id) REFERENCES asset.gantangan(id, arena_id) ON DELETE RESTRICT,
    CHECK (status <> 'CONFIRMED' OR gantangan_id IS NOT NULL)
);
CREATE UNIQUE INDEX registration_items_active_bird_uidx ON competition.registration_items (round_id, bird_id)
    WHERE status IN ('DRAFT','PENDING_PAYMENT','CONFIRMED');
CREATE UNIQUE INDEX registration_items_active_gantangan_uidx ON competition.registration_items (round_id, gantangan_id)
    WHERE status IN ('DRAFT','PENDING_PAYMENT','CONFIRMED') AND gantangan_id IS NOT NULL;
CREATE INDEX registration_items_registration_idx ON competition.registration_items (registration_id, arena_id);
CREATE INDEX registration_items_bird_idx ON competition.registration_items (bird_id);
CREATE INDEX registration_items_gantangan_idx ON competition.registration_items (gantangan_id, arena_id);
CREATE TABLE competition.registration_payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    registration_id uuid NOT NULL,
    amount numeric(12,2) NOT NULL CHECK (amount > 0),
    method varchar(20) NOT NULL CHECK (method IN ('CASH','TRANSFER')),
    proof_reference text,
    status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (status IN ('PENDING','VERIFIED','REJECTED')),
    verified_by_user_id uuid REFERENCES iam.users(id) ON DELETE RESTRICT,
    verified_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (registration_id, arena_id) REFERENCES competition.registrations(id, arena_id) ON DELETE RESTRICT,
    CHECK ((status = 'VERIFIED' AND verified_by_user_id IS NOT NULL AND verified_at IS NOT NULL)
        OR (status <> 'VERIFIED' AND verified_by_user_id IS NULL AND verified_at IS NULL))
);
CREATE UNIQUE INDEX registration_payments_verified_uidx ON competition.registration_payments (registration_id)
    WHERE status = 'VERIFIED';
CREATE INDEX registration_payments_registration_idx ON competition.registration_payments (registration_id, arena_id);

CREATE TABLE judging.score_criteria (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    criteria_name varchar(100) NOT NULL CHECK (btrim(criteria_name) <> ''),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (arena_id, criteria_name),
    UNIQUE (id, arena_id)
);
COMMENT ON TABLE judging.score_criteria IS
    'Provisional ERD assumption: shared criteria per arena; scope and locking policy still require review.';
CREATE TABLE judging.round_judges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    round_id uuid NOT NULL,
    judge_user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE RESTRICT,
    UNIQUE (round_id, judge_user_id),
    UNIQUE (id, round_id, arena_id),
    FOREIGN KEY (round_id, arena_id) REFERENCES competition.rounds(id, arena_id) ON DELETE RESTRICT
);
CREATE INDEX round_judges_user_idx ON judging.round_judges (judge_user_id);
CREATE TABLE judging.round_scores (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id) ON DELETE RESTRICT,
    round_id uuid NOT NULL,
    registration_item_id uuid NOT NULL,
    round_judge_id uuid NOT NULL,
    criteria_id uuid NOT NULL,
    score_value numeric NOT NULL CHECK (score_value BETWEEN 1 AND 10 AND score_value = trunc(score_value)),
    revision_number integer NOT NULL DEFAULT 1 CHECK (revision_number > 0),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (registration_item_id, round_judge_id, criteria_id),
    FOREIGN KEY (registration_item_id, round_id, arena_id)
        REFERENCES competition.registration_items(id, round_id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (round_judge_id, round_id, arena_id)
        REFERENCES judging.round_judges(id, round_id, arena_id) ON DELETE RESTRICT,
    FOREIGN KEY (criteria_id, arena_id) REFERENCES judging.score_criteria(id, arena_id) ON DELETE RESTRICT
);
CREATE INDEX round_scores_round_idx ON judging.round_scores (round_id, arena_id);
CREATE INDEX round_scores_judge_idx ON judging.round_scores (round_judge_id, round_id, arena_id);
CREATE INDEX round_scores_criteria_idx ON judging.round_scores (criteria_id, arena_id);
CREATE TABLE judging.score_revisions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    score_id uuid NOT NULL REFERENCES judging.round_scores(id) ON DELETE RESTRICT,
    operation_id uuid NOT NULL,
    revision_number integer NOT NULL CHECK (revision_number > 0),
    score_value numeric NOT NULL CHECK (score_value BETWEEN 1 AND 10 AND score_value = trunc(score_value)),
    submitted_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (score_id, revision_number),
    UNIQUE (score_id, operation_id)
);

CREATE TABLE media.round_recordings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_item_id uuid NOT NULL UNIQUE REFERENCES competition.registration_items(id) ON DELETE RESTRICT,
    video_url text,
    audio_url text,
    max_peak_dbfs numeric(5,2),
    avg_rms_dbfs numeric(5,2),
    clip_incident_count integer NOT NULL DEFAULT 0 CHECK (clip_incident_count >= 0),
    singing_duration_seconds integer NOT NULL DEFAULT 0 CHECK (singing_duration_seconds >= 0),
    recording_status varchar(20) NOT NULL DEFAULT 'PENDING'
        CHECK (recording_status IN ('PENDING','RECORDING','FINALIZING','READY','UPLOADED','FAILED','INTERRUPTED')),
    delivery_status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (delivery_status IN ('PENDING','SENT','FAILED')),
    recipient_phone_snapshot varchar(30),
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (delivery_status <> 'SENT' OR sent_at IS NOT NULL
        AND recipient_phone_snapshot IS NOT NULL AND btrim(recipient_phone_snapshot) <> '')
);
CREATE INDEX round_recordings_delivery_idx ON media.round_recordings (delivery_status, created_at, id);
COMMENT ON TABLE media.round_recordings IS 'Written by engine/worker services; delivery contact is snapshotted from registration, not bird owner.';
-- No bracket tables, next-round link, promotion relationship, or automatic winner selection.
-- Phase transitions, scoring windows, required-score completeness, role checks,
-- ownership transfer audit, payment confirmation, criteria/roster locking and
-- the three-device limit remain coordinated by application UseCases.
