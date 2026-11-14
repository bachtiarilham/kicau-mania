-- Read models for migration 008 and ERD 2026-11-14. Run in one transaction.
-- No legacy item/payment/assessment aliases: the old application must be adapted.
CREATE SCHEMA cloud_read;
REVOKE ALL ON SCHEMA cloud_read FROM PUBLIC;

CREATE VIEW cloud_read.v_users AS
SELECT id, name, phone, email, email_verified_at, is_active, is_eo,
       password_hash IS NOT NULL AS has_password, created_at, updated_at
FROM iam.users;
CREATE VIEW cloud_read.v_roles AS SELECT id, name FROM iam.roles;
CREATE VIEW cloud_read.v_user_roles AS SELECT id, user_id, role_id FROM iam.user_roles;
CREATE VIEW cloud_read.v_arena_user_roles AS SELECT id, arena_id, user_id, role_id FROM iam.arena_user_roles;
CREATE VIEW cloud_read.v_user_sessions AS SELECT id, user_id, device_id, expires_at, revoked_at, user_agent FROM iam.user_sessions;
CREATE VIEW cloud_read.v_user_auth_providers AS SELECT id, user_id, provider FROM iam.user_auth_providers;

CREATE VIEW cloud_read.v_arenas AS
SELECT id, name, address, timezone, is_active, criteria_locked_at, created_at, updated_at FROM asset.arenas;
CREATE VIEW cloud_read.v_gantangan AS
SELECT g.id, g.arena_id, a.name AS arena_name, g.number, g.description, g.baris, g.kolom
FROM asset.gantangan g JOIN asset.arenas a ON a.id = g.arena_id;
CREATE VIEW cloud_read.v_bird_species AS SELECT id, name, created_at, updated_at FROM asset.bird_species;
CREATE VIEW cloud_read.v_birds AS
SELECT b.id, b.bird_code, b.name, b.owner_id, u.name AS owner_name,
       b.bird_species_id, s.name AS bird_species_name, b.created_at, b.updated_at
FROM asset.birds b JOIN iam.users u ON u.id = b.owner_id
JOIN asset.bird_species s ON s.id = b.bird_species_id;
CREATE VIEW cloud_read.v_bird_ownership_history AS
SELECT h.id, h.bird_id, b.bird_code, b.name AS bird_name, h.user_id,
       u.name AS owner_name, h.ownership_date, h.notes
FROM asset.bird_ownership_history h JOIN asset.birds b ON b.id = h.bird_id
JOIN iam.users u ON u.id = h.user_id;

CREATE VIEW cloud_read.v_categories AS
SELECT id, arena_id, code, name FROM competition.categories;
CREATE VIEW cloud_read.v_events AS
WITH counts AS (SELECT event_id, count(*) AS round_count FROM competition.rounds GROUP BY event_id)
SELECT e.id, e.arena_id, a.name AS arena_name, e.owner_id, u.name AS organizer_name,
       e.title, e.description, e.event_date, e.status, e.created_at, e.updated_at,
       coalesce(c.round_count, 0) AS round_count
FROM competition.events e JOIN asset.arenas a ON a.id = e.arena_id
JOIN iam.users u ON u.id = e.owner_id LEFT JOIN counts c ON c.event_id = e.id;
CREATE VIEW cloud_read.v_rounds AS
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
CREATE VIEW cloud_read.v_registrations AS
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
COMMENT ON VIEW cloud_read.v_registrations IS 'Private operational view. Payment proof bytes are intentionally excluded; enforce actor/arena access before returning data.';
CREATE VIEW cloud_read.v_bird_award_certificate_history AS
SELECT h.id, h.bird_id, b.bird_code, b.name AS bird_name, h.registration_id,
       t.arena_id, t.round_id, r.event_id, h.award_name,
       CASE WHEN h.registration_id IS NULL THEN 'EXTERNAL' ELSE 'INTERNAL' END AS source_type,
       CASE WHEN h.registration_id IS NULL THEN h.event_name ELSE e.title END AS event_name,
       CASE WHEN h.registration_id IS NULL THEN h.organizer_name ELSE eo.name END AS organizer_name,
       h.rank, h.awarded_at, h.certificate_url IS NOT NULL AS has_certificate,
       h.created_at, h.updated_at
FROM asset.bird_award_certificate_history h JOIN asset.birds b ON b.id = h.bird_id
LEFT JOIN competition.registrations t ON t.id = h.registration_id
LEFT JOIN competition.rounds r ON r.id = t.round_id
LEFT JOIN competition.events e ON e.id = r.event_id
LEFT JOIN iam.users eo ON eo.id = e.owner_id;
CREATE VIEW cloud_read.v_eo_arena_history AS
SELECT e.owner_id, u.name AS organizer_name, e.arena_id, a.name AS arena_name,
       count(*) AS event_count,
       count(*) FILTER (WHERE e.status IN ('ACTIVE','COMPLETED')) AS held_event_count,
       count(*) FILTER (WHERE e.status = 'SCHEDULED') AS scheduled_event_count,
       count(*) FILTER (WHERE e.status = 'CANCELLED') AS cancelled_event_count,
       min(e.event_date) FILTER (WHERE e.status IN ('ACTIVE','COMPLETED')) AS first_held_event_date,
       max(e.event_date) FILTER (WHERE e.status IN ('ACTIVE','COMPLETED')) AS last_held_event_date
FROM competition.events e JOIN iam.users u ON u.id = e.owner_id
JOIN asset.arenas a ON a.id = e.arena_id
GROUP BY e.owner_id, u.name, e.arena_id, a.name;

CREATE VIEW cloud_read.v_score_criteria AS SELECT id, arena_id, criteria_name FROM judging.score_criteria;
CREATE VIEW cloud_read.v_round_judges AS
SELECT j.id, r.arena_id, j.round_id, j.judge_user_id, u.name AS judge_name
FROM judging.round_judges j JOIN competition.rounds r ON r.id = j.round_id
JOIN iam.users u ON u.id = j.judge_user_id;
CREATE VIEW cloud_read.v_round_scores AS
SELECT s.id, s.arena_id, s.round_id, s.registration_id, s.user_id,
       u.name AS judge_name, s.criteria_id, c.criteria_name, s.score_value, s.created_at, s.updated_at
FROM judging.round_scores s JOIN iam.users u ON u.id = s.user_id
JOIN judging.score_criteria c ON c.id = s.criteria_id;
CREATE VIEW cloud_read.v_round_recordings AS
SELECT m.id, m.registration_id, t.arena_id, t.round_id, t.user_id AS recipient_user_id,
       u.phone AS recipient_phone, m.recording_status, m.delivery_status, m.sent_at,
       m.video_url IS NOT NULL AS has_video, m.audio_url IS NOT NULL AS has_audio,
       m.created_at, m.updated_at
FROM media.round_recordings m JOIN competition.registrations t ON t.id = m.registration_id
JOIN iam.users u ON u.id = t.user_id;

CREATE VIEW cloud_read.v_round_score_summary AS
WITH totals AS (
    SELECT registration_id, count(*) AS score_count, count(DISTINCT user_id) AS scoring_judge_count,
           sum(score_value) AS total_score, avg(score_value) AS average_score
    FROM judging.round_scores GROUP BY registration_id
), judges AS (
    SELECT round_id, count(*) AS judge_count FROM judging.round_judges GROUP BY round_id
), criteria AS (
    SELECT arena_id, count(*) AS criteria_count FROM judging.score_criteria GROUP BY arena_id
)
SELECT t.id AS registration_id, t.arena_id, r.event_id, t.round_id, t.bird_id, t.gantangan_id,
       coalesce(s.score_count, 0) AS score_count, coalesce(s.scoring_judge_count, 0) AS scoring_judge_count,
       coalesce(j.judge_count, 0) * coalesce(c.criteria_count, 0) AS expected_score_count,
       coalesce(j.judge_count, 0) AS judge_count, coalesce(c.criteria_count, 0) AS criteria_count,
       s.total_score, s.average_score,
       (coalesce(j.judge_count, 0) > 0 AND coalesce(c.criteria_count, 0) > 0 AND
        coalesce(s.score_count, 0) = j.judge_count * c.criteria_count) AS is_complete
FROM competition.registrations t JOIN competition.rounds r ON r.id = t.round_id
LEFT JOIN totals s ON s.registration_id = t.id
LEFT JOIN judges j ON j.round_id = t.round_id LEFT JOIN criteria c ON c.arena_id = t.arena_id
WHERE t.status = 'CONFIRMED' AND t.payment_status = 'VERIFIED' AND t.participant_status = 'ACTIVE';
COMMENT ON VIEW cloud_read.v_round_score_summary IS 'Live current-score averages, including decimal scores. Disqualified/cancelled/unconfirmed participants are excluded. No official winner is persisted by this schema.';

CREATE MATERIALIZED VIEW cloud_read.mv_round_score_summary AS
SELECT registration_id, arena_id, event_id, round_id, bird_id, gantangan_id, score_count,
       scoring_judge_count, expected_score_count, judge_count, criteria_count,
       total_score, average_score, is_complete FROM cloud_read.v_round_score_summary WITH DATA;
CREATE UNIQUE INDEX mv_score_registration_uidx ON cloud_read.mv_round_score_summary(registration_id);
CREATE INDEX mv_score_round_idx ON cloud_read.mv_round_score_summary(arena_id, round_id, registration_id);
CREATE MATERIALIZED VIEW cloud_read.mv_arena_dashboard AS
WITH events AS (SELECT arena_id, count(*) AS event_count FROM competition.events GROUP BY arena_id),
rounds AS (
    SELECT arena_id, count(*) AS round_count, count(*) FILTER (WHERE status = 'CLOSED') AS closed_round_count
    FROM competition.rounds GROUP BY arena_id
), tickets AS (
    SELECT arena_id, count(*) AS registration_count,
           count(*) FILTER (WHERE status = 'CONFIRMED' AND payment_status = 'VERIFIED' AND participant_status = 'ACTIVE') AS confirmed_participant_count,
           count(*) FILTER (WHERE participant_status = 'DISQUALIFIED') AS disqualified_count,
           sum(entry_fee) FILTER (WHERE payment_status = 'VERIFIED') AS verified_payment_amount
    FROM competition.registrations GROUP BY arena_id
)
SELECT a.id AS arena_id, coalesce(e.event_count, 0) AS event_count, coalesce(r.round_count, 0) AS round_count,
       coalesce(r.closed_round_count, 0) AS closed_round_count, coalesce(t.registration_count, 0) AS registration_count,
       coalesce(t.confirmed_participant_count, 0) AS confirmed_participant_count,
       coalesce(t.disqualified_count, 0) AS disqualified_count,
       coalesce(t.verified_payment_amount, 0) AS verified_payment_amount
FROM asset.arenas a LEFT JOIN events e ON e.arena_id = a.id
LEFT JOIN rounds r ON r.arena_id = a.id LEFT JOIN tickets t ON t.arena_id = a.id WITH DATA;
CREATE UNIQUE INDEX mv_dashboard_arena_uidx ON cloud_read.mv_arena_dashboard(arena_id);
CREATE TABLE cloud_read.refresh_status (
    view_name text PRIMARY KEY,
    snapshot_at timestamptz NOT NULL,
    refreshed_at timestamptz NOT NULL
);
INSERT INTO cloud_read.refresh_status VALUES
('mv_round_score_summary', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
('mv_arena_dashboard', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

GRANT USAGE ON SCHEMA cloud_read TO kicau_cloud_reader, kicau_cloud_admin_reader, kicau_cloud_recap_maintainer, kicau_cloud_app;
GRANT SELECT ON ALL TABLES IN SCHEMA cloud_read TO kicau_cloud_admin_reader, kicau_cloud_app;
GRANT SELECT ON cloud_read.v_arenas, cloud_read.v_gantangan, cloud_read.v_bird_species,
 cloud_read.v_birds, cloud_read.v_categories, cloud_read.v_events, cloud_read.v_rounds TO kicau_cloud_reader;
GRANT MAINTAIN ON cloud_read.mv_round_score_summary, cloud_read.mv_arena_dashboard TO kicau_cloud_recap_maintainer;
GRANT SELECT, UPDATE ON cloud_read.refresh_status TO kicau_cloud_recap_maintainer;
GRANT kicau_cloud_recap_maintainer TO kicau_cloud_app;
REVOKE INSERT, UPDATE, DELETE ON iam.roles FROM kicau_cloud_app;
