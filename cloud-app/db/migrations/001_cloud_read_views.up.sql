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
