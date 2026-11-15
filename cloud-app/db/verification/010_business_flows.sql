-- Scenario executed by run_business_flows.py; named query bodies are loaded from db/queries/*.sql.
-- All fixtures are rolled back. Run on an isolated PostgreSQL test database after 010.
BEGIN;
SET LOCAL ROLE kicau_cloud_app;
EXECUTE setmutationcontext('cf67bd2e-6c63-47f0-bbf7-f091f867279c',NULL);
EXECUTE participantcreate('cf67bd2e-6c63-47f0-bbf7-f091f867279c','Flow administrator','+909002222001','flow-admin@example.test','argon2id-fixture-not-a-real-password');
EXECUTE assignglobalrole('cf67bd2e-6c63-47f0-bbf7-f091f867279c','ADMIN');
EXECUTE participantassignrole('cf67bd2e-6c63-47f0-bbf7-f091f867279c');
EXECUTE masteruserupdate('cf67bd2e-6c63-47f0-bbf7-f091f867279c',1,'{"is_eo":true}');
EXECUTE participantcreate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1','Flow owner','+909002222002','flow-owner@example.test',NULL);
EXECUTE participantassignrole('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1');
EXECUTE participantcreate('7cb5d560-4f7e-4b29-90fa-795332fb28a8','Flow judge','+909002222003','flow-judge@example.test',NULL);
EXECUTE assignglobalrole('7cb5d560-4f7e-4b29-90fa-795332fb28a8','JURI');
SELECT set_config('flow.owner_updated_at',(SELECT updated_at::text FROM iam.users WHERE email='flow-owner@example.test'),true);
EXECUTE masteruserupdate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',1,'{"name":"Flow owner"}');
DO $$ BEGIN IF NOT ((SELECT version=1 FROM iam.users WHERE id='8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1')) THEN RAISE EXCEPTION 'No-op user edit changed version'; END IF; END $$;
DO $$ BEGIN IF (SELECT updated_at FROM iam.users WHERE email='flow-owner@example.test') IS DISTINCT FROM current_setting('flow.owner_updated_at')::timestamptz THEN RAISE EXCEPTION 'No-op edit changed updated_at'; END IF; END $$;
EXECUTE masteruserupdate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',1,'{"name":"Flow owner edited"}');
EXECUTE masteruserupdate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',2,'{"name":"Flow owner"}');
EXECUTE masteruserupdate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',1,'{"name":"STALE"}');
DO $$ BEGIN IF NOT ((SELECT version=3 AND name='Flow owner' FROM iam.users WHERE id='8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1')) THEN RAISE EXCEPTION 'Version conflict or A-B-A failed'; END IF; END $$;
DO $$ BEGIN IF NOT ((SELECT count(*)=3 FROM iam.user_master_history WHERE user_id='8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1')) THEN RAISE EXCEPTION 'Wrong master history count'; END IF; END $$;
EXECUTE loginaccountbyemail('FLOW-ADMIN@EXAMPLE.TEST');
EXECUTE loginuserlock('cf67bd2e-6c63-47f0-bbf7-f091f867279c');
EXECUTE loginotherdevicecount('cf67bd2e-6c63-47f0-bbf7-f091f867279c','fixture-device');
EXECUTE loginsessionsave('cf67bd2e-6c63-47f0-bbf7-f091f867279c','fixture-device',CURRENT_TIMESTAMP+interval '1 day','Verification agent');
SELECT id AS session FROM iam.user_sessions WHERE user_id='cf67bd2e-6c63-47f0-bbf7-f091f867279c' \gset
SELECT set_config('flow.session',:'session',true);
EXECUTE loginsessionvalidate(:'session','cf67bd2e-6c63-47f0-bbf7-f091f867279c');
EXECUTE logingooglelink('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1','fixture-verified-google-subject');
EXECUTE logingoogleaccount('fixture-verified-google-subject');
EXECUTE arenacreate('Flow arena',NULL,'Asia/Jakarta',true);
SELECT id AS arena FROM asset.arenas WHERE name='Flow arena' \gset
SELECT set_config('flow.arena',:'arena',true);
EXECUTE setmutationcontext('cf67bd2e-6c63-47f0-bbf7-f091f867279c',:'arena');
EXECUTE assignarenarole('7cb5d560-4f7e-4b29-90fa-795332fb28a8',:'arena','JURI');
EXECUTE birdspeciescreate('Flow species');
SELECT id AS species FROM asset.bird_species WHERE name='Flow species' \gset
SELECT set_config('flow.species',:'species',true);
EXECUTE birdcreate('Flow bird','cf67bd2e-6c63-47f0-bbf7-f091f867279c',:'species');
SELECT id AS bird FROM asset.birds WHERE name='Flow bird' \gset
SELECT set_config('flow.bird',:'bird',true);
EXECUTE birdtransferowner(:'bird',1,'8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1','Transfer fixture');
EXECUTE birdtransferowner(:'bird',2,'8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1','No-op transfer');
DO $$ BEGIN IF NOT ((SELECT version=2 FROM asset.birds WHERE id=current_setting('flow.bird')::uuid)) THEN RAISE EXCEPTION 'No-op transfer changed bird version'; END IF; END $$;
DO $$ BEGIN IF NOT ((SELECT count(*)=2 FROM asset.bird_ownership_history WHERE bird_id=current_setting('flow.bird')::uuid)) THEN RAISE EXCEPTION 'Transfer did not write ownership history exactly once'; END IF; END $$;
EXECUTE birdupdate(:'bird',1,'{"name":"STALE BIRD"}');
EXECUTE birdget(:'bird');
EXECUTE birdmasterhistory(:'bird',0,100);
EXECUTE gantangancreate(:'arena',1,NULL,1,1);
SELECT id AS gantangan FROM asset.gantangan WHERE arena_id=:'arena' AND number=1 \gset
SELECT set_config('flow.gantangan',:'gantangan',true);
EXECUTE gantanganupdate(:'arena',:'gantangan','{"description":"Flow fixture"}');
EXECUTE categorycreate(:'arena','FLOW','Flow category');
SELECT id AS category FROM competition.categories WHERE arena_id=:'arena' AND code='FLOW' \gset
SELECT set_config('flow.category',:'category',true);
EXECUTE categoryupdate(:'arena',:'category','{"name":"Flow category updated"}');
EXECUTE eventcreate(:'arena','cf67bd2e-6c63-47f0-bbf7-f091f867279c','Flow event',NULL,CURRENT_DATE,'SCHEDULED');
SELECT id AS event FROM competition.events WHERE title='Flow event' \gset
SELECT set_config('flow.event',:'event',true);
EXECUTE roundcreate(:'arena',:'event',:'category',:'species',1,'Flow round');
SELECT id AS round FROM competition.rounds WHERE event_id=:'event' AND round_number=1 \gset
SELECT set_config('flow.round',:'round',true);
EXECUTE roundcreate(:'arena',:'event',NULL,:'species',2,'Flow round 2');
SELECT id AS round2 FROM competition.rounds WHERE event_id=:'event' AND round_number=2 \gset
SELECT set_config('flow.round2',:'round2',true);
EXECUTE registrationcreate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',:'arena',:'round',:'bird',:'gantangan',50000,'TRANSFER');
SELECT id AS ticket FROM competition.registrations WHERE round_id=:'round' AND bird_id=:'bird' \gset
SELECT set_config('flow.ticket',:'ticket',true);
EXECUTE registrationsetproof(:'arena',:'ticket',decode('89504e47','hex'),'image/png');
EXECUTE registrationverifypayment(:'arena',:'ticket');
EXECUTE scorecriterioncreate(:'arena','Flow criterion');
SELECT id AS criterion FROM judging.score_criteria WHERE arena_id=:'arena' AND criteria_name='Flow criterion' \gset
SELECT set_config('flow.criterion',:'criterion',true);
EXECUTE roundjudgeassign(:'arena',:'round','7cb5d560-4f7e-4b29-90fa-795332fb28a8');
EXECUTE arenalock(:'arena');
EXECUTE roundlock(:'arena',:'round');
EXECUTE roundsetstatus(:'arena',:'round','PREPARED','RUNNING');
EXECUTE arenalockcriteria(:'arena');
EXECUTE roundsetstatus(:'arena',:'round','RUNNING','JUDGING');
EXECUTE scoreupsert(:'arena',:'round',:'ticket','7cb5d560-4f7e-4b29-90fa-795332fb28a8',:'criterion',8.5);
EXECUTE scoreupsert(:'arena',:'round',:'ticket','7cb5d560-4f7e-4b29-90fa-795332fb28a8',:'criterion',9.25);
EXECUTE roundlivesummary(:'arena',:'round');
EXECUTE roundcompleteness(:'arena',:'round');
DO $$ BEGIN IF NOT ((SELECT count(*)=1 AND min(score_value)=9.25 FROM judging.round_scores WHERE registration_id=current_setting('flow.ticket')::uuid)) THEN RAISE EXCEPTION 'Score revision created a duplicate or lost decimals'; END IF; END $$;
EXECUTE birdawardcreate(:'bird',:'ticket','Flow award',NULL,NULL,1,CURRENT_DATE,'https://example.test/award.pdf');
SELECT id AS award FROM asset.bird_award_certificate_history WHERE registration_id=:'ticket' \gset
SELECT set_config('flow.award',:'award',true);
EXECUTE birdawardupdate(:'bird',:'award','Flow award updated',NULL,NULL,1,CURRENT_DATE,NULL);
EXECUTE birdawardlist(:'bird',NULL,100);
EXECUTE mediarecordingsave(:'ticket','https://example.test/video.mp4',NULL,'COMPLETED');
SELECT id AS recording FROM media.round_recordings WHERE registration_id=:'ticket' \gset
SELECT set_config('flow.recording',:'recording',true);
EXECUTE mediarecordingdelivery(:'recording','SENT');
EXECUTE mediarecordingget(:'arena',:'recording');
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_arena_dashboard;
EXECUTE reportroundsnapshot(:'arena',:'round');
EXECUTE reportarenadashboard(:'arena');
EXECUTE participantdisqualify(:'arena',:'ticket','Flow disqualification');
EXECUTE scoreupsert(:'arena',:'round',:'ticket','7cb5d560-4f7e-4b29-90fa-795332fb28a8',:'criterion',3);
DO $$ BEGIN IF NOT ((SELECT score_value=9.25 FROM judging.round_scores WHERE registration_id=current_setting('flow.ticket')::uuid)) THEN RAISE EXCEPTION 'Disqualified score was changed'; END IF; END $$;
DO $$ BEGIN IF NOT (NOT EXISTS(SELECT FROM cloud_read.v_round_score_summary WHERE registration_id=current_setting('flow.ticket')::uuid)) THEN RAISE EXCEPTION 'Disqualified ticket remained eligible'; END IF; END $$;
EXECUTE birdsoftdelete(:'bird',2);
EXECUTE registrationcreate('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',:'arena',:'round2',:'bird',:'gantangan',50000,'CASH');
DO $$ BEGIN IF NOT (NOT EXISTS(SELECT FROM competition.registrations WHERE round_id=current_setting('flow.round2')::uuid)) THEN RAISE EXCEPTION 'Deleted bird was offered to new registration'; END IF; END $$;
DO $$ BEGIN IF NOT (EXISTS(SELECT FROM cloud_read.v_registrations WHERE id=current_setting('flow.ticket')::uuid)) THEN RAISE EXCEPTION 'Old ticket lost its master relationship'; END IF; END $$;
EXECUTE loginsessionsave('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1','owner-device',CURRENT_TIMESTAMP+interval '1 day',NULL);
EXECUTE masterusersoftdelete('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',3);
EXECUTE masterusersoftdelete('8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1',4);
DO $$ BEGIN IF NOT ((SELECT version=4 AND NOT is_active AND deleted_at IS NOT NULL FROM iam.users WHERE id='8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1')) THEN RAISE EXCEPTION 'Soft-delete version/idempotency failed'; END IF; END $$;
DO $$ BEGIN IF NOT (NOT EXISTS(SELECT FROM iam.user_sessions WHERE user_id='8dc66ef0-c62b-4bf3-b4e4-eb15e6697ef1' AND revoked_at IS NULL)) THEN RAISE EXCEPTION 'Soft delete did not revoke sessions'; END IF; END $$;
EXECUTE loginaccountbyemail('flow-owner@example.test');
DO $$ BEGIN BEGIN EXECUTE 'UPDATE iam.user_master_history SET version=version+1'; RAISE EXCEPTION 'History UPDATE was permitted'; EXCEPTION WHEN SQLSTATE '42501' THEN NULL; END; END $$;
DO $$ BEGIN BEGIN EXECUTE 'TRUNCATE iam.user_master_history'; RAISE EXCEPTION 'History TRUNCATE was permitted'; EXCEPTION WHEN SQLSTATE '42501' THEN NULL; END; END $$;
DO $$ BEGIN BEGIN EXECUTE 'DELETE FROM asset.bird_master_history'; RAISE EXCEPTION 'History DELETE was permitted'; EXCEPTION WHEN SQLSTATE '42501' THEN NULL; END; END $$;
DO $$ BEGIN BEGIN EXECUTE 'INSERT INTO iam.user_master_history(user_id,version,snapshot,change_type) SELECT user_id,99,snapshot,''BASELINE'' FROM iam.user_master_history LIMIT 1'; RAISE EXCEPTION 'History INSERT was permitted'; EXCEPTION WHEN SQLSTATE '42501' THEN NULL; END; END $$;
DO $$ BEGIN BEGIN EXECUTE 'DELETE FROM iam.users'; RAISE EXCEPTION 'Physical user delete was permitted'; EXCEPTION WHEN SQLSTATE '42501' THEN NULL; END; END $$;
SELECT set_config('app.actor_id','',true);
DO $$ BEGIN BEGIN EXECUTE 'INSERT INTO iam.users(name) VALUES(''Missing actor'')'; RAISE EXCEPTION 'Actorless master creation was permitted'; EXCEPTION WHEN SQLSTATE '23514' THEN NULL; END; END $$;
EXECUTE setmutationcontext('0e3a1bc0-e03a-47db-889c-cff5d2cea9c9',NULL);
DO $$ BEGIN BEGIN EXECUTE 'UPDATE iam.users SET name=''Must rollback'' WHERE id=''7cb5d560-4f7e-4b29-90fa-795332fb28a8'''; RAISE EXCEPTION 'Missing history actor did not abort master update'; EXCEPTION WHEN SQLSTATE '23503' THEN NULL; END; END $$;
DO $$ BEGIN IF NOT ((SELECT name='Flow judge' AND version=1 FROM iam.users WHERE id='7cb5d560-4f7e-4b29-90fa-795332fb28a8')) THEN RAISE EXCEPTION 'Master survived a failed history insert'; END IF; END $$;
SELECT 'PASS: named query business flow, versions/no-op/conflict, owner transfer, scoring, disqualification, soft delete/session revocation, and protected atomic history';
ROLLBACK;
