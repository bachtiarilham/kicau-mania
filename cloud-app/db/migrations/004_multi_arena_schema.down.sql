-- Roll back 005 first. Empty database only; prevents loss of operational data.
-- Rebuild the exact legacy business tables, then restore the 001..003 read layer.
DO $empty$
DECLARE table_name text; occupied boolean;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['asset.arenas', 'asset.bird_owner_transfers', 'asset.birds', 'asset.devices', 'asset.engines', 'asset.gantangan', 'asset.owners', 'competition.categories', 'competition.events', 'competition.registration_items', 'competition.registration_payments', 'competition.registrations', 'competition.rounds', 'iam.arena_user_roles', 'iam.roles', 'iam.user_auth_providers', 'iam.user_roles', 'iam.user_sessions', 'iam.users', 'judging.round_judges', 'judging.round_scores', 'judging.score_criteria', 'judging.score_revisions', 'media.round_recordings', 'member.plans', 'member.user_subscriptions']
    LOOP
        EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE', table_name);
        EXECUTE format('SELECT EXISTS (SELECT FROM %s)', table_name) INTO occupied;
        IF occupied THEN
            RAISE EXCEPTION 'Migration requires empty tables; % contains data. No changes applied.', table_name;
        END IF;
    END LOOP;
END;
$empty$;

DO $winner$
BEGIN
    IF EXISTS (SELECT FROM information_schema.columns WHERE table_schema = 'competition'
        AND table_name = 'rounds' AND column_name = 'winner_registration_item_id') THEN
        RAISE EXCEPTION 'Roll back 005 before 004';
    END IF;
END;
$winner$;
DROP TABLE media.round_recordings;
DROP TABLE judging.score_revisions;
DROP TABLE judging.round_scores;
DROP TABLE judging.round_judges;
DROP TABLE judging.score_criteria;
DROP TABLE competition.registration_payments;
DROP TABLE competition.registration_items;
DROP TABLE competition.registrations;
DROP TABLE competition.rounds;
DROP TABLE competition.categories;
DROP TABLE competition.events;
DROP TABLE asset.bird_owner_transfers;
DROP TABLE asset.birds;
DROP TABLE asset.owners;
DROP TABLE asset.devices;
DROP TABLE asset.engines;
DROP TABLE asset.gantangan;
DROP TABLE iam.arena_user_roles;
DROP TABLE iam.user_roles;
DROP TABLE asset.arenas;

DROP INDEX iam.user_sessions_device_active_uidx;
ALTER TABLE iam.user_sessions DROP COLUMN device_id;
ALTER TABLE iam.users ADD COLUMN role_id uuid NOT NULL REFERENCES iam.roles(id) ON DELETE RESTRICT;

CREATE TABLE asset.bird_owners (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    bird_id uuid NOT NULL,
    user_id uuid NOT NULL,
    is_primary_contact boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE asset.birds (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    ring_number character varying(50),
    bird_type character varying(50) NOT NULL,
    name character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE asset.devices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    type character varying(20) NOT NULL,
    ip_address character varying(45) NOT NULL,
    rtsp_url text NOT NULL,
    lapak_id uuid,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE asset.lapaks (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    number integer NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE competition.events (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    title character varying(150) NOT NULL,
    date date NOT NULL,
    location text NOT NULL,
    status character varying(20) DEFAULT 'scheduled'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE competition.round_participants (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    round_id uuid NOT NULL,
    lapak_id uuid NOT NULL,
    bird_id uuid NOT NULL,
    registered_by_user_id uuid NOT NULL,
    registered_as_role character varying(20) NOT NULL,
    declared_owner_user_id uuid,
    declared_owner_phone character varying(20),
    owner_consent_status character varying(30) DEFAULT 'approved'::character varying,
    legal_declaration_agreed boolean DEFAULT false NOT NULL,
    target_recipient_phone character varying(20) NOT NULL,
    status character varying(20) DEFAULT 'confirmed'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE competition.rounds (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    event_id uuid NOT NULL,
    round_number integer NOT NULL,
    bird_type character varying(50) NOT NULL,
    start_time timestamp with time zone,
    end_time timestamp with time zone,
    status character varying(20) DEFAULT 'pending'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE judging.round_scores (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    round_participant_id uuid NOT NULL,
    judge_user_id uuid NOT NULL,
    criteria_id uuid NOT NULL,
    score_value numeric(6,2) DEFAULT 0.00 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE judging.score_criteria (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    criteria_name character varying(100) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE media.round_recordings (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    round_participant_id uuid NOT NULL,
    video_url text NOT NULL,
    audio_url text,
    max_peak_dbfs numeric(5,2),
    avg_rms_dbfs numeric(5,2),
    clip_incident_count integer DEFAULT 0,
    singing_duration_seconds integer DEFAULT 0,
    is_sent boolean DEFAULT false,
    sent_at timestamp with time zone,
    sent_to_phone character varying(20),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE ONLY asset.bird_owners
    ADD CONSTRAINT bird_owners_bird_id_user_id_key UNIQUE (bird_id, user_id);

ALTER TABLE ONLY asset.bird_owners
    ADD CONSTRAINT bird_owners_pkey PRIMARY KEY (id);

ALTER TABLE ONLY asset.birds
    ADD CONSTRAINT birds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY asset.birds
    ADD CONSTRAINT birds_ring_number_key UNIQUE (ring_number);

ALTER TABLE ONLY asset.devices
    ADD CONSTRAINT devices_pkey PRIMARY KEY (id);

ALTER TABLE ONLY asset.lapaks
    ADD CONSTRAINT lapaks_number_key UNIQUE (number);

ALTER TABLE ONLY asset.lapaks
    ADD CONSTRAINT lapaks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY competition.events
    ADD CONSTRAINT events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_pkey PRIMARY KEY (id);

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_round_id_lapak_id_key UNIQUE (round_id, lapak_id);

ALTER TABLE ONLY competition.rounds
    ADD CONSTRAINT rounds_event_id_round_number_key UNIQUE (event_id, round_number);

ALTER TABLE ONLY competition.rounds
    ADD CONSTRAINT rounds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY judging.round_scores
    ADD CONSTRAINT round_scores_pkey PRIMARY KEY (id);

ALTER TABLE ONLY judging.round_scores
    ADD CONSTRAINT round_scores_round_participant_id_judge_user_id_criteria_id_key UNIQUE (round_participant_id, judge_user_id, criteria_id);

ALTER TABLE ONLY judging.score_criteria
    ADD CONSTRAINT score_criteria_criteria_name_key UNIQUE (criteria_name);

ALTER TABLE ONLY judging.score_criteria
    ADD CONSTRAINT score_criteria_pkey PRIMARY KEY (id);

ALTER TABLE ONLY media.round_recordings
    ADD CONSTRAINT round_recordings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY media.round_recordings
    ADD CONSTRAINT round_recordings_round_participant_id_key UNIQUE (round_participant_id);

ALTER TABLE ONLY asset.bird_owners
    ADD CONSTRAINT bird_owners_bird_id_fkey FOREIGN KEY (bird_id) REFERENCES asset.birds(id) ON DELETE CASCADE;

ALTER TABLE ONLY asset.bird_owners
    ADD CONSTRAINT bird_owners_user_id_fkey FOREIGN KEY (user_id) REFERENCES iam.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY asset.devices
    ADD CONSTRAINT devices_lapak_id_fkey FOREIGN KEY (lapak_id) REFERENCES asset.lapaks(id) ON DELETE SET NULL;

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_bird_id_fkey FOREIGN KEY (bird_id) REFERENCES asset.birds(id) ON DELETE RESTRICT;

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_declared_owner_user_id_fkey FOREIGN KEY (declared_owner_user_id) REFERENCES iam.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_lapak_id_fkey FOREIGN KEY (lapak_id) REFERENCES asset.lapaks(id) ON DELETE RESTRICT;

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_registered_by_user_id_fkey FOREIGN KEY (registered_by_user_id) REFERENCES iam.users(id) ON DELETE RESTRICT;

ALTER TABLE ONLY competition.round_participants
    ADD CONSTRAINT round_participants_round_id_fkey FOREIGN KEY (round_id) REFERENCES competition.rounds(id) ON DELETE CASCADE;

ALTER TABLE ONLY competition.rounds
    ADD CONSTRAINT rounds_event_id_fkey FOREIGN KEY (event_id) REFERENCES competition.events(id) ON DELETE CASCADE;

ALTER TABLE ONLY judging.round_scores
    ADD CONSTRAINT round_scores_criteria_id_fkey FOREIGN KEY (criteria_id) REFERENCES judging.score_criteria(id) ON DELETE RESTRICT;

ALTER TABLE ONLY judging.round_scores
    ADD CONSTRAINT round_scores_judge_user_id_fkey FOREIGN KEY (judge_user_id) REFERENCES iam.users(id) ON DELETE RESTRICT;

ALTER TABLE ONLY judging.round_scores
    ADD CONSTRAINT round_scores_round_participant_id_fkey FOREIGN KEY (round_participant_id) REFERENCES competition.round_participants(id) ON DELETE CASCADE;

ALTER TABLE ONLY media.round_recordings
    ADD CONSTRAINT round_recordings_round_participant_id_fkey FOREIGN KEY (round_participant_id) REFERENCES competition.round_participants(id) ON DELETE CASCADE;

CREATE INDEX idx_round_participants_lookup ON competition.round_participants USING btree (round_id, lapak_id);

CREATE INDEX idx_round_scores_participant ON judging.round_scores USING btree (round_participant_id);

CREATE INDEX idx_media_recordings_sent ON media.round_recordings USING btree (is_sent);

-- Restore the previous read-layer state without modifying migration history.
CREATE SCHEMA cloud_read;
REVOKE ALL ON SCHEMA cloud_read FROM PUBLIC;

CREATE VIEW cloud_read.v_events AS
SELECT e.id, e.title, e.date, e.location, e.status, e.created_at,
       COALESCE(rc.round_count, 0) AS round_count
FROM competition.events AS e
LEFT JOIN (
    SELECT event_id, count(*) AS round_count
    FROM competition.rounds GROUP BY event_id
) AS rc ON rc.event_id = e.id;

CREATE VIEW cloud_read.v_rounds AS
SELECT r.id, r.event_id, r.round_number, r.bird_type, r.start_time, r.end_time,
       r.status, r.created_at, COALESCE(pc.participant_count, 0) AS participant_count
FROM competition.rounds AS r
LEFT JOIN (
    SELECT round_id, count(*) AS participant_count
    FROM competition.round_participants GROUP BY round_id
) AS pc ON pc.round_id = r.id;

CREATE VIEW cloud_read.v_round_participants AS
SELECT p.id, r.event_id, p.round_id, p.bird_id, b.name AS bird_name,
       p.lapak_id, l.number AS lapak_number, p.registered_by_user_id,
       p.registered_as_role, p.status, p.owner_consent_status, p.created_at
FROM competition.round_participants AS p
JOIN competition.rounds AS r ON r.id = p.round_id
JOIN asset.birds AS b ON b.id = p.bird_id
JOIN asset.lapaks AS l ON l.id = p.lapak_id;

CREATE VIEW cloud_read.v_birds AS
SELECT b.id, b.ring_number, b.name, b.bird_type, b.created_at,
       COALESCE(oc.owner_count, 0) AS owner_count
FROM asset.birds AS b
LEFT JOIN (
    SELECT bird_id, count(*) AS owner_count
    FROM asset.bird_owners GROUP BY bird_id
) AS oc ON oc.bird_id = b.id;

CREATE VIEW cloud_read.v_bird_owners AS
SELECT o.id, o.bird_id, o.user_id, u.name AS owner_name,
       o.is_primary_contact, o.created_at
FROM asset.bird_owners AS o
JOIN iam.users AS u ON u.id = o.user_id;

CREATE VIEW cloud_read.v_lapaks AS
SELECT l.id, l.number, l.description, l.created_at,
       COALESCE(dc.device_count, 0) AS device_count
FROM asset.lapaks AS l
LEFT JOIN (
    SELECT lapak_id, count(*) AS device_count
    FROM asset.devices WHERE lapak_id IS NOT NULL GROUP BY lapak_id
) AS dc ON dc.lapak_id = l.id;

CREATE VIEW cloud_read.v_devices AS
SELECT d.id, d.type, d.ip_address, d.lapak_id, l.number AS lapak_number, d.created_at
FROM asset.devices AS d
LEFT JOIN asset.lapaks AS l ON l.id = d.lapak_id;

CREATE VIEW cloud_read.v_users AS
SELECT u.id, u.name, u.role_id, r.name AS role_name,
       u.is_active, u.is_verified, u.created_at, u.updated_at
FROM iam.users AS u
JOIN iam.roles AS r ON r.id = u.role_id;

CREATE VIEW cloud_read.v_roles AS
SELECT id, name, created_at FROM iam.roles;

CREATE VIEW cloud_read.v_user_auth_providers AS
SELECT id, user_id, provider, created_at FROM iam.user_auth_providers;

CREATE VIEW cloud_read.v_user_sessions AS
SELECT id, user_id, expires_at, revoked_at, created_at FROM iam.user_sessions;

CREATE VIEW cloud_read.v_plans AS
SELECT id, name, quota_birds, max_events, price, created_at FROM member.plans;

CREATE VIEW cloud_read.v_user_subscriptions AS
SELECT s.id, s.user_id, s.plan_id, p.name AS plan_name, s.start_date, s.end_date,
       s.status, s.created_at, (s.status = 'active' AND s.start_date <= CURRENT_TIMESTAMP
           AND CURRENT_TIMESTAMP < s.end_date) AS is_current
FROM member.user_subscriptions AS s
JOIN member.plans AS p ON p.id = s.plan_id;

CREATE VIEW cloud_read.v_score_criteria AS
SELECT id, criteria_name, created_at FROM judging.score_criteria;

CREATE VIEW cloud_read.v_round_scores AS
SELECT s.id, r.event_id, p.round_id, s.round_participant_id, s.judge_user_id,
       s.criteria_id, c.criteria_name, s.score_value, s.created_at
FROM judging.round_scores AS s
JOIN judging.score_criteria AS c ON c.id = s.criteria_id
JOIN competition.round_participants AS p ON p.id = s.round_participant_id
JOIN competition.rounds AS r ON r.id = p.round_id;

CREATE VIEW cloud_read.v_round_recordings AS
SELECT m.id, r.event_id, p.round_id, m.round_participant_id,
       m.max_peak_dbfs, m.avg_rms_dbfs, m.clip_incident_count, m.singing_duration_seconds,
       m.is_sent, m.sent_at, m.created_at
FROM media.round_recordings AS m
JOIN competition.round_participants AS p ON p.id = m.round_participant_id
JOIN competition.rounds AS r ON r.id = p.round_id;

CREATE MATERIALIZED VIEW cloud_read.mv_round_score_summary AS
WITH classified_scores AS (
    SELECT round_participant_id, judge_user_id, score_value,
           score_value BETWEEN 1 AND 10 AND score_value = trunc(score_value) AS is_valid
    FROM judging.round_scores
),
score_totals AS (
    SELECT round_participant_id,
           count(*) AS submitted_score_count,
           count(*) FILTER (WHERE is_valid) AS valid_score_count,
           count(*) FILTER (WHERE NOT is_valid) AS invalid_score_count,
           count(DISTINCT judge_user_id) FILTER (WHERE is_valid) AS scoring_judge_count,
           avg(score_value) FILTER (WHERE is_valid) AS average_score
    FROM classified_scores
    GROUP BY round_participant_id
)
SELECT r.event_id, p.round_id, p.id AS round_participant_id, p.lapak_id, p.bird_id,
       COALESCE(s.submitted_score_count, 0) AS submitted_score_count,
       COALESCE(s.valid_score_count, 0) AS valid_score_count,
       COALESCE(s.invalid_score_count, 0) AS invalid_score_count,
       COALESCE(s.scoring_judge_count, 0) AS scoring_judge_count,
       s.average_score
FROM competition.round_participants AS p
JOIN competition.rounds AS r ON r.id = p.round_id
LEFT JOIN score_totals AS s ON s.round_participant_id = p.id
WITH DATA;

CREATE UNIQUE INDEX mv_round_score_summary_participant_uidx
    ON cloud_read.mv_round_score_summary (round_participant_id);
CREATE INDEX mv_round_score_summary_round_participant_idx
    ON cloud_read.mv_round_score_summary (round_id, round_participant_id);

COMMENT ON MATERIALIZED VIEW cloud_read.mv_round_score_summary IS
    'Administrative snapshot of available scores. Refresh after committed imports or corrections.';

-- Group roles are cluster-wide. Create them only when missing.
DO $roles$
DECLARE
    role_name text;
BEGIN
    FOREACH role_name IN ARRAY ARRAY[
        'kicau_cloud_reader', 'kicau_cloud_admin_reader', 'kicau_cloud_recap_maintainer'
    ]
    LOOP
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = role_name) THEN
            EXECUTE format('CREATE ROLE %I NOLOGIN', role_name);
        ELSIF EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = role_name AND rolcanlogin) THEN
            RAISE EXCEPTION 'Expected NOLOGIN group role: %', role_name;
        END IF;
    END LOOP;
END;
$roles$;

REVOKE ALL ON SCHEMA cloud_read FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA cloud_read FROM PUBLIC;
GRANT USAGE ON SCHEMA cloud_read TO kicau_cloud_reader, kicau_cloud_admin_reader, kicau_cloud_recap_maintainer;

GRANT SELECT ON
    cloud_read.v_events,
    cloud_read.v_rounds,
    cloud_read.v_round_participants,
    cloud_read.v_birds,
    cloud_read.v_bird_owners,
    cloud_read.v_lapaks,
    cloud_read.v_plans,
    cloud_read.v_user_subscriptions,
    cloud_read.v_score_criteria,
    cloud_read.v_round_recordings
TO kicau_cloud_reader, kicau_cloud_admin_reader;

GRANT SELECT ON
    cloud_read.v_devices,
    cloud_read.v_users,
    cloud_read.v_roles,
    cloud_read.v_user_auth_providers,
    cloud_read.v_user_sessions,
    cloud_read.v_round_scores
TO kicau_cloud_admin_reader;
GRANT SELECT ON cloud_read.mv_round_score_summary TO kicau_cloud_admin_reader;
GRANT MAINTAIN ON cloud_read.mv_round_score_summary TO kicau_cloud_recap_maintainer;

