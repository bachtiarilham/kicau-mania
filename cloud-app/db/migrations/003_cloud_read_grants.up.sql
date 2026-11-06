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
