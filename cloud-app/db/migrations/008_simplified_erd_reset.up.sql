-- DESTRUCTIVE, explicit development reset approved for the 2026-11-14 ERD.
-- Run with psql -X -v ON_ERROR_STOP=1 -1. All business data is erased.
-- This does not install Redis or make the previous Go/SQL application compatible.
DO $precondition$
BEGIN
    IF to_regclass('judging.participant_assessments') IS NULL
       OR to_regclass('cloud_read.refresh_status') IS NULL
       OR to_regclass('asset.bird_species') IS NOT NULL THEN
        RAISE EXCEPTION '008 requires the old schema after 007; it must not be repeated';
    END IF;
END;
$precondition$;

DROP SCHEMA cloud_read CASCADE;
DROP SCHEMA media CASCADE;
DROP SCHEMA judging CASCADE;
DROP SCHEMA competition CASCADE;
DROP SCHEMA member CASCADE;
DROP SCHEMA asset CASCADE;
DROP SCHEMA iam CASCADE;

CREATE SCHEMA iam;
CREATE SCHEMA asset;
CREATE SCHEMA competition;
CREATE SCHEMA judging;
CREATE SCHEMA media;
REVOKE ALL ON SCHEMA iam, asset, competition, judging, media FROM PUBLIC;

CREATE TABLE iam.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    phone varchar(30) UNIQUE CHECK (phone IS NULL OR btrim(phone) <> ''),
    email varchar(254) CHECK (email IS NULL OR btrim(email) <> ''),
    password_hash text,
    email_verified_at timestamptz,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_eo boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX users_email_normalized_uidx ON iam.users (lower(btrim(email))) WHERE email IS NOT NULL;
CREATE TABLE iam.roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(50) NOT NULL UNIQUE CHECK (btrim(name) <> '')
);
-- Configuration only: no users, passwords, arenas or fictional event data.
INSERT INTO iam.roles(name) VALUES ('SUPERADMIN'), ('OPERATOR'), ('JURI'), ('EO_ADMIN'), ('PESERTA');
CREATE TABLE iam.user_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    role_id uuid NOT NULL REFERENCES iam.roles(id),
    UNIQUE (user_id, role_id)
);
CREATE INDEX user_roles_role_idx ON iam.user_roles(role_id);
CREATE TABLE iam.user_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    device_id varchar(200) NOT NULL CHECK (btrim(device_id) <> ''),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    user_agent text
);
CREATE UNIQUE INDEX user_sessions_device_active_uidx ON iam.user_sessions(user_id, device_id) WHERE revoked_at IS NULL;
CREATE INDEX user_sessions_user_expiry_idx ON iam.user_sessions(user_id, expires_at) WHERE revoked_at IS NULL;
COMMENT ON TABLE iam.user_sessions IS 'ERD device sessions; three-active-device limit and future token storage are application responsibilities. Previous hash-token queries are incompatible.';
CREATE TABLE iam.user_auth_providers (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    provider varchar(30) NOT NULL CHECK (provider = 'google'),
    provider_user_id text NOT NULL UNIQUE CHECK (btrim(provider_user_id) <> ''),
    UNIQUE (user_id, provider)
);
COMMENT ON TABLE iam.users IS 'EO flag is separate from authorization. Email verification/reset tokens belong in Redis in the new design; Redis is not installed by this migration.';

CREATE TABLE asset.arenas (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    address text,
    timezone varchar(100) NOT NULL DEFAULT 'Asia/Jakarta' CHECK (btrim(timezone) <> ''),
    is_active boolean NOT NULL DEFAULT true,
    criteria_locked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE iam.arena_user_roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    role_id uuid NOT NULL REFERENCES iam.roles(id),
    UNIQUE (arena_id, user_id, role_id)
);
CREATE INDEX arena_user_roles_user_idx ON iam.arena_user_roles(user_id, arena_id);
CREATE INDEX arena_user_roles_role_idx ON iam.arena_user_roles(role_id);
CREATE TABLE asset.gantangan (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    number integer NOT NULL CHECK (number > 0),
    description text,
    baris integer NOT NULL CHECK (baris > 0),
    kolom integer NOT NULL CHECK (kolom > 0),
    UNIQUE (arena_id, number),
    UNIQUE (arena_id, baris, kolom),
    UNIQUE (id, arena_id)
);
CREATE TABLE asset.bird_species (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name varchar(100) NOT NULL UNIQUE CHECK (btrim(name) <> ''),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE SEQUENCE asset.bird_code_seq;
CREATE FUNCTION asset.new_bird_code() RETURNS text LANGUAGE sql VOLATILE
SET search_path = pg_catalog, asset AS $code$
    SELECT 'BRD-' || lpad(n::text, greatest(6, length(n::text)), '0')
    FROM (SELECT nextval('asset.bird_code_seq') AS n) generated;
$code$;
REVOKE ALL ON FUNCTION asset.new_bird_code() FROM PUBLIC;
CREATE TABLE asset.birds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bird_code varchar(100) NOT NULL DEFAULT asset.new_bird_code() UNIQUE CHECK (btrim(bird_code) <> ''),
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    owner_id uuid NOT NULL REFERENCES iam.users(id),
    bird_species_id uuid NOT NULL REFERENCES asset.bird_species(id),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX birds_owner_idx ON asset.birds(owner_id, id);
CREATE INDEX birds_species_idx ON asset.birds(bird_species_id, id);
COMMENT ON COLUMN asset.birds.bird_code IS 'Stable global identifier. Sequence default is concurrency-safe; owner/name updates must preserve the code.';
CREATE TABLE asset.bird_ownership_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bird_id uuid NOT NULL REFERENCES asset.birds(id),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    ownership_date timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes text
);
CREATE INDEX bird_ownership_history_bird_idx ON asset.bird_ownership_history(bird_id, ownership_date, id);
CREATE INDEX bird_ownership_history_user_idx ON asset.bird_ownership_history(user_id);

CREATE TABLE competition.events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    owner_id uuid NOT NULL REFERENCES iam.users(id),
    title varchar(200) NOT NULL CHECK (btrim(title) <> ''),
    description text,
    event_date date NOT NULL,
    status varchar(20) NOT NULL DEFAULT 'SCHEDULED' CHECK (status IN ('SCHEDULED','ACTIVE','COMPLETED','CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (id, arena_id)
);
CREATE INDEX events_arena_date_idx ON competition.events(arena_id, event_date, id);
CREATE INDEX events_owner_arena_idx ON competition.events(owner_id, arena_id, event_date, id);
COMMENT ON COLUMN competition.events.owner_id IS 'EO account. UseCase must validate is_eo and authorization when creating an event; the flag does not grant arena permissions.';
CREATE TABLE competition.categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    code varchar(50) NOT NULL CHECK (btrim(code) <> ''),
    name varchar(150) NOT NULL CHECK (btrim(name) <> ''),
    UNIQUE (arena_id, code),
    UNIQUE (id, arena_id)
);
CREATE TABLE competition.rounds (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bird_species_id uuid NOT NULL REFERENCES asset.bird_species(id),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    event_id uuid NOT NULL,
    category_id uuid,
    round_number integer NOT NULL CHECK (round_number > 0),
    title varchar(200) NOT NULL CHECK (btrim(title) <> ''),
    status varchar(20) NOT NULL DEFAULT 'PREPARED' CHECK (status IN ('PREPARED','RUNNING','JUDGING','CLOSED','CANCELLED')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (arena_id, event_id, round_number),
    UNIQUE (id, arena_id),
    FOREIGN KEY (event_id, arena_id) REFERENCES competition.events(id, arena_id),
    FOREIGN KEY (category_id, arena_id) REFERENCES competition.categories(id, arena_id)
);
CREATE INDEX rounds_species_idx ON competition.rounds(bird_species_id);
CREATE INDEX rounds_category_idx ON competition.rounds(category_id, arena_id);
COMMENT ON TABLE competition.rounds IS 'Independent rounds, manual bracket outside the application. No stored winner, confirmer, fee or duration in this ERD.';
CREATE TABLE competition.registrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES iam.users(id),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    round_id uuid NOT NULL,
    bird_id uuid NOT NULL REFERENCES asset.birds(id),
    gantangan_id uuid NOT NULL,
    entry_fee numeric(14,2) NOT NULL CHECK (entry_fee BETWEEN 0 AND 999999999999.99),
    payment_method varchar(20) NOT NULL CHECK (payment_method IN ('CASH','TRANSFER')),
    payment_status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (payment_status IN ('PENDING','VERIFIED','REJECTED')),
    payment_proof bytea,
    payment_proof_mime varchar(100),
    verified_at timestamptz,
    status varchar(30) NOT NULL DEFAULT 'PENDING_PAYMENT' CHECK (status IN ('PENDING_PAYMENT','CONFIRMED','CANCELLED')),
    participant_status varchar(20) NOT NULL DEFAULT 'ACTIVE' CHECK (participant_status IN ('ACTIVE','DISQUALIFIED')),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    disqualification_reason text,
    disqualified_at timestamptz,
    UNIQUE (round_id, bird_id),
    UNIQUE (id, round_id, arena_id),
    UNIQUE (id, bird_id),
    FOREIGN KEY (round_id, arena_id) REFERENCES competition.rounds(id, arena_id),
    FOREIGN KEY (gantangan_id, arena_id) REFERENCES asset.gantangan(id, arena_id),
    CHECK ((payment_proof IS NULL AND payment_proof_mime IS NULL) OR
           (payment_proof IS NOT NULL AND octet_length(payment_proof) > 0 AND payment_proof_mime LIKE 'image/%')),
    CHECK ((payment_status = 'VERIFIED') = (verified_at IS NOT NULL)),
    CHECK (status <> 'CONFIRMED' OR payment_status = 'VERIFIED'),
    CHECK ((participant_status = 'ACTIVE' AND disqualification_reason IS NULL AND disqualified_at IS NULL) OR
           (participant_status = 'DISQUALIFIED' AND disqualification_reason IS NOT NULL AND btrim(disqualification_reason) <> '' AND disqualified_at IS NOT NULL))
);
CREATE UNIQUE INDEX registrations_confirmed_gantangan_uidx ON competition.registrations(round_id, gantangan_id) WHERE status = 'CONFIRMED';
CREATE INDEX registrations_user_idx ON competition.registrations(user_id, created_at, id);
CREATE INDEX registrations_arena_round_idx ON competition.registrations(arena_id, round_id, id);
CREATE INDEX registrations_bird_idx ON competition.registrations(bird_id);
CREATE INDEX registrations_gantangan_idx ON competition.registrations(gantangan_id, arena_id);
COMMENT ON TABLE competition.registrations IS 'One ticket = one bird, round and gantangan. Disqualification does not cancel registration/payment or release a confirmed gantangan. Species equality and eligibility are UseCase checks.';

CREATE TABLE asset.bird_award_certificate_history (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bird_id uuid NOT NULL REFERENCES asset.birds(id),
    registration_id uuid,
    award_name varchar(200) NOT NULL CHECK (btrim(award_name) <> ''),
    event_name varchar(200),
    organizer_name varchar(200),
    rank integer CHECK (rank > 0),
    awarded_at date NOT NULL,
    certificate_url text CHECK (certificate_url IS NULL OR btrim(certificate_url) <> ''),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (registration_id, bird_id) REFERENCES competition.registrations(id, bird_id),
    CHECK (registration_id IS NOT NULL OR (event_name IS NOT NULL AND btrim(event_name) <> ''))
);
CREATE INDEX bird_awards_bird_date_idx ON asset.bird_award_certificate_history(bird_id, awarded_at, id);
CREATE INDEX bird_awards_registration_idx ON asset.bird_award_certificate_history(registration_id, bird_id);
COMMENT ON TABLE asset.bird_award_certificate_history IS 'Internal/external awards. UseCase must reject internal awards for disqualified participants; certificates are URLs, not database blobs.';

CREATE TABLE judging.score_criteria (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    criteria_name varchar(150) NOT NULL CHECK (btrim(criteria_name) <> ''),
    UNIQUE (arena_id, criteria_name),
    UNIQUE (id, arena_id)
);
CREATE TABLE judging.round_judges (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    round_id uuid NOT NULL REFERENCES competition.rounds(id),
    judge_user_id uuid NOT NULL REFERENCES iam.users(id),
    UNIQUE (round_id, judge_user_id)
);
CREATE INDEX round_judges_user_idx ON judging.round_judges(judge_user_id, round_id);
CREATE TABLE judging.round_scores (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    arena_id uuid NOT NULL REFERENCES asset.arenas(id),
    round_id uuid NOT NULL,
    registration_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES iam.users(id),
    criteria_id uuid NOT NULL,
    score_value numeric NOT NULL CHECK (score_value BETWEEN 1 AND 10),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_judge_round_criteria UNIQUE (round_id, registration_id, user_id, criteria_id),
    FOREIGN KEY (round_id, arena_id) REFERENCES competition.rounds(id, arena_id),
    FOREIGN KEY (registration_id, round_id, arena_id) REFERENCES competition.registrations(id, round_id, arena_id),
    FOREIGN KEY (round_id, user_id) REFERENCES judging.round_judges(round_id, judge_user_id),
    FOREIGN KEY (criteria_id, arena_id) REFERENCES judging.score_criteria(id, arena_id)
);
CREATE INDEX round_scores_registration_idx ON judging.round_scores(registration_id, round_id, arena_id);
CREATE INDEX round_scores_user_idx ON judging.round_scores(user_id);
CREATE INDEX round_scores_criteria_idx ON judging.round_scores(criteria_id, arena_id);
COMMENT ON TABLE judging.round_scores IS 'Current scores only, integer or decimal 1..10. UseCase handles JUDGING-only submit/revision, disqualification, locking and updated_at. No audit/version/idempotency ledger.';
CREATE TABLE media.round_recordings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    registration_id uuid NOT NULL UNIQUE REFERENCES competition.registrations(id),
    video_url text,
    audio_url text,
    recording_status varchar(20) NOT NULL DEFAULT 'RECORDING' CHECK (recording_status IN ('RECORDING','COMPLETED','FAILED')),
    delivery_status varchar(20) NOT NULL DEFAULT 'PENDING' CHECK (delivery_status IN ('PENDING','SENT','FAILED')),
    sent_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ((delivery_status = 'SENT') = (sent_at IS NOT NULL))
);
COMMENT ON TABLE media.round_recordings IS 'Metadata only; recipient phone is joined through registrations.user_id -> iam.users.phone. This migration does not activate engine ingest or messaging.';

GRANT USAGE ON SCHEMA iam, asset, competition, judging, media TO kicau_cloud_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA iam, asset, competition, judging, media TO kicau_cloud_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA asset TO kicau_cloud_app;
GRANT EXECUTE ON FUNCTION asset.new_bird_code() TO kicau_cloud_app;
