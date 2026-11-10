CREATE SCHEMA cloud_read;
REVOKE ALL ON SCHEMA cloud_read FROM PUBLIC;
CREATE VIEW cloud_read.v_users AS SELECT id,name,email,phone,is_active,is_verified,email_verified_at,created_at,password_hash IS NOT NULL AS has_password FROM iam.users;
CREATE VIEW cloud_read.v_user_auth_providers AS SELECT id,user_id,provider,created_at FROM iam.user_auth_providers;
CREATE VIEW cloud_read.v_roles AS SELECT id,name,created_at FROM iam.roles;
CREATE VIEW cloud_read.v_user_roles AS SELECT id,user_id,role_id FROM iam.user_roles;
CREATE VIEW cloud_read.v_arena_user_roles AS SELECT id,arena_id,user_id,role_id FROM iam.arena_user_roles;
CREATE VIEW cloud_read.v_user_sessions AS SELECT id,user_id,device_id,user_agent,ip_address,expires_at,revoked_at,created_at,updated_at FROM iam.user_sessions;
CREATE VIEW cloud_read.v_arenas AS SELECT id,name,address,timezone,is_active,criteria_locked_at,created_at FROM asset.arenas;
CREATE VIEW cloud_read.v_gantangan AS SELECT id,arena_id,number,description,created_at FROM asset.gantangan;
CREATE VIEW cloud_read.v_engines AS SELECT id,arena_id,name,is_active,last_heartbeat_at,created_at FROM asset.engines;
CREATE VIEW cloud_read.v_devices AS SELECT id,arena_id,engine_id,gantangan_id,type,created_at FROM asset.devices;
CREATE VIEW cloud_read.v_owners AS SELECT id,user_id,name,phone,created_by_user_id,created_in_arena_id,created_at FROM asset.owners;
CREATE VIEW cloud_read.v_birds AS SELECT id,owner_id,ring_number,bird_type,name,created_at FROM asset.birds;
CREATE VIEW cloud_read.v_bird_owner_transfers AS SELECT id,bird_id,from_owner_id,to_owner_id,performed_by_user_id,reason,transferred_at FROM asset.bird_owner_transfers;
CREATE VIEW cloud_read.v_events AS SELECT id,arena_id,title,date,location,status,created_at FROM competition.events;
CREATE VIEW cloud_read.v_categories AS SELECT id,arena_id,code,name,created_at FROM competition.categories;
CREATE VIEW cloud_read.v_rounds AS SELECT id,arena_id,event_id,category_id,round_number,bird_type,entry_fee,phase,interrupted_from_phase,session_duration_minutes,judging_duration_minutes,start_time,end_time,winner_registration_item_id,winner_confirmed_by_user_id,winner_confirmed_at,created_at FROM competition.rounds;
CREATE VIEW cloud_read.v_registrations AS SELECT id,arena_id,registered_by_user_id,created_by_user_id,registration_source,registrant_name,registrant_phone,total_amount,status,created_at FROM competition.registrations;
CREATE VIEW cloud_read.v_registration_items AS
SELECT i.id,i.arena_id,i.registration_id,i.round_id,r.event_id,r.round_number,i.bird_id,b.name AS bird_name,b.ring_number,
 i.gantangan_id,g.number AS gantangan_number,i.entry_fee_snapshot,i.status,i.created_at
FROM competition.registration_items i JOIN competition.rounds r ON r.id=i.round_id
JOIN asset.birds b ON b.id=i.bird_id LEFT JOIN asset.gantangan g ON g.id=i.gantangan_id;
CREATE VIEW cloud_read.v_registration_payments AS SELECT id,arena_id,registration_id,amount,method,proof_reference,status,verified_by_user_id,verified_at,created_at FROM competition.registration_payments;
CREATE VIEW cloud_read.v_score_criteria AS SELECT id,arena_id,criteria_name,created_at FROM judging.score_criteria;
CREATE VIEW cloud_read.v_round_judges AS SELECT id,arena_id,round_id,judge_user_id FROM judging.round_judges;
CREATE VIEW cloud_read.v_round_scores AS SELECT id,arena_id,round_id,registration_item_id,round_judge_id,criteria_id,score_value,revision_number,created_at FROM judging.round_scores;
CREATE VIEW cloud_read.v_score_revisions AS SELECT id,score_id,operation_id,revision_number,score_value,submitted_at FROM judging.score_revisions;
CREATE VIEW cloud_read.v_participant_assessments AS SELECT id,arena_id,round_id,registration_item_id,round_judge_id,version,updated_at FROM judging.participant_assessments;
CREATE VIEW cloud_read.v_round_recordings AS SELECT id,registration_item_id,recording_status,delivery_status,max_peak_dbfs,avg_rms_dbfs,clip_incident_count,singing_duration_seconds,sent_at,created_at FROM media.round_recordings;

CREATE VIEW cloud_read.v_round_score_summary AS
WITH totals AS (
 SELECT registration_item_id,count(*) score_count,count(DISTINCT round_judge_id) scoring_judge_count,
 sum(score_value) total_score,avg(score_value) average_score
 FROM judging.round_scores GROUP BY registration_item_id
), judges AS (SELECT round_id,count(*) judge_count FROM judging.round_judges GROUP BY round_id),
 criteria AS (SELECT arena_id,count(*) criteria_count FROM judging.score_criteria GROUP BY arena_id)
SELECT i.id registration_item_id,i.arena_id,r.event_id,i.round_id,i.bird_id,i.gantangan_id,
 coalesce(t.score_count,0) score_count,coalesce(t.scoring_judge_count,0) scoring_judge_count,
 coalesce(j.judge_count,0)*coalesce(c.criteria_count,0) expected_score_count,
 coalesce(j.judge_count,0) judge_count,coalesce(c.criteria_count,0) criteria_count,t.total_score,t.average_score
FROM competition.registration_items i JOIN competition.rounds r ON r.id=i.round_id
LEFT JOIN totals t ON t.registration_item_id=i.id LEFT JOIN judges j ON j.round_id=i.round_id
LEFT JOIN criteria c ON c.arena_id=i.arena_id WHERE i.status='CONFIRMED';
CREATE MATERIALIZED VIEW cloud_read.mv_round_score_summary AS
 SELECT registration_item_id,arena_id,event_id,round_id,bird_id,gantangan_id,score_count,scoring_judge_count,
 expected_score_count,judge_count,criteria_count,total_score,average_score FROM cloud_read.v_round_score_summary WITH DATA;
CREATE UNIQUE INDEX mv_score_item_uidx ON cloud_read.mv_round_score_summary(registration_item_id);
CREATE INDEX mv_score_round_idx ON cloud_read.mv_round_score_summary(arena_id,round_id,registration_item_id);
CREATE MATERIALIZED VIEW cloud_read.mv_arena_dashboard AS
WITH e AS(SELECT arena_id,count(*) event_count FROM competition.events GROUP BY arena_id),
 r AS(SELECT arena_id,count(*) round_count,count(*) FILTER(WHERE phase='CLOSED') closed_round_count FROM competition.rounds GROUP BY arena_id),
 g AS(SELECT arena_id,count(*) registration_count FROM competition.registrations GROUP BY arena_id),
 i AS(SELECT arena_id,count(*) confirmed_participant_count FROM competition.registration_items WHERE status='CONFIRMED' GROUP BY arena_id),
 p AS(SELECT arena_id,sum(amount) verified_payment_amount FROM competition.registration_payments WHERE status='VERIFIED' GROUP BY arena_id)
SELECT a.id arena_id,coalesce(e.event_count,0) event_count,coalesce(r.round_count,0) round_count,
 coalesce(r.closed_round_count,0) closed_round_count,coalesce(g.registration_count,0) registration_count,
 coalesce(i.confirmed_participant_count,0) confirmed_participant_count,coalesce(p.verified_payment_amount,0) verified_payment_amount
FROM asset.arenas a LEFT JOIN e ON e.arena_id=a.id LEFT JOIN r ON r.arena_id=a.id LEFT JOIN g ON g.arena_id=a.id
LEFT JOIN i ON i.arena_id=a.id LEFT JOIN p ON p.arena_id=a.id WITH DATA;
CREATE UNIQUE INDEX mv_dashboard_arena_uidx ON cloud_read.mv_arena_dashboard(arena_id);
CREATE TABLE cloud_read.refresh_status(view_name text PRIMARY KEY,snapshot_at timestamptz NOT NULL,refreshed_at timestamptz NOT NULL);
INSERT INTO cloud_read.refresh_status VALUES ('mv_round_score_summary',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),('mv_arena_dashboard',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP);
GRANT USAGE ON SCHEMA cloud_read TO kicau_cloud_reader,kicau_cloud_admin_reader,kicau_cloud_recap_maintainer;
GRANT SELECT ON ALL TABLES IN SCHEMA cloud_read TO kicau_cloud_admin_reader;
GRANT SELECT ON cloud_read.v_arenas,cloud_read.v_categories,cloud_read.v_gantangan,cloud_read.v_events,cloud_read.v_rounds,cloud_read.v_birds TO kicau_cloud_reader;
GRANT MAINTAIN ON cloud_read.mv_round_score_summary,cloud_read.mv_arena_dashboard TO kicau_cloud_recap_maintainer;
GRANT SELECT,UPDATE ON cloud_read.refresh_status TO kicau_cloud_recap_maintainer;

DO $application_role$
BEGIN
 IF NOT EXISTS(SELECT FROM pg_roles WHERE rolname='kicau_cloud_app') THEN
  CREATE ROLE kicau_cloud_app NOLOGIN;
 ELSIF EXISTS(SELECT FROM pg_roles WHERE rolname='kicau_cloud_app' AND rolcanlogin) THEN
  RAISE EXCEPTION 'Expected NOLOGIN group role kicau_cloud_app';
 END IF;
END;
$application_role$;
GRANT USAGE ON SCHEMA iam,asset,competition,judging,cloud_read TO kicau_cloud_app;
GRANT SELECT ON ALL TABLES IN SCHEMA cloud_read TO kicau_cloud_app;
GRANT SELECT ON iam.roles TO kicau_cloud_app;
GRANT SELECT,INSERT,UPDATE ON iam.users,iam.user_sessions,iam.user_auth_providers,iam.account_tokens TO kicau_cloud_app;
GRANT SELECT,INSERT,UPDATE,DELETE ON iam.user_roles,iam.arena_user_roles,
 asset.arenas,asset.gantangan,asset.engines,asset.devices,asset.owners,asset.birds,
 competition.events,competition.categories,competition.rounds,competition.registrations,competition.registration_items,competition.registration_payments,
 judging.score_criteria,judging.round_judges TO kicau_cloud_app;
GRANT SELECT,INSERT ON asset.bird_owner_transfers,judging.score_revisions,judging.score_operations TO kicau_cloud_app;
GRANT SELECT,INSERT,UPDATE ON judging.round_scores,judging.participant_assessments TO kicau_cloud_app;
GRANT kicau_cloud_recap_maintainer TO kicau_cloud_app;
