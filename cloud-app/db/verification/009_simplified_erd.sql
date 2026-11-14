-- PostgreSQL integration checks, not Go tests. No fixture data is committed.
-- Run after 008 + 009 with psql -X -v ON_ERROR_STOP=1 -f this-file.sql.
BEGIN;
SET LOCAL ROLE kicau_cloud_app;
DO $verify$
DECLARE
    eo uuid; judge1 uuid; judge2 uuid; registrant uuid; owner2 uuid;
    arena1 uuid; arena2 uuid; species uuid; bird1 uuid; bird2 uuid; bird3 uuid;
    event1 uuid; round1 uuid; round2 uuid; gantangan1 uuid; gantangan2 uuid; gantangan3 uuid;
    ticket1 uuid; ticket2 uuid; criterion1 uuid; criterion2 uuid; foreign_criterion uuid;
    code1 text; code2 text; result record; first_score uuid;
BEGIN
    INSERT INTO iam.users(name,is_eo) VALUES('Verification EO',true) RETURNING id INTO eo;
    INSERT INTO iam.users(name) VALUES('Verification judge 1') RETURNING id INTO judge1;
    INSERT INTO iam.users(name) VALUES('Verification judge 2') RETURNING id INTO judge2;
    INSERT INTO iam.users(name,phone) VALUES('Verification registrant','+909009999001') RETURNING id INTO registrant;
    INSERT INTO iam.users(name) VALUES('Verification second owner') RETURNING id INTO owner2;
    INSERT INTO asset.arenas(name) VALUES('Verification arena 1') RETURNING id INTO arena1;
    INSERT INTO asset.arenas(name) VALUES('Verification arena 2') RETURNING id INTO arena2;
    INSERT INTO asset.bird_species(name) VALUES('Verification species') RETURNING id INTO species;
    INSERT INTO asset.birds(name,owner_id,bird_species_id) VALUES('Verification bird 1',eo,species) RETURNING id,bird_code INTO bird1,code1;
    INSERT INTO asset.birds(name,owner_id,bird_species_id) VALUES('Verification bird 2',owner2,species) RETURNING id,bird_code INTO bird2,code2;
    INSERT INTO asset.birds(name,owner_id,bird_species_id) VALUES('Verification bird 3',eo,species) RETURNING id INTO bird3;
    IF code1 = code2 OR code1 !~ '^BRD-[0-9]{6,}$' THEN RAISE EXCEPTION 'Bird code generation failed'; END IF;
    UPDATE asset.birds SET owner_id=owner2,updated_at=CURRENT_TIMESTAMP WHERE id=bird1;
    IF (SELECT bird_code FROM asset.birds WHERE id=bird1) <> code1 THEN RAISE EXCEPTION 'Ownership changed bird code'; END IF;
    INSERT INTO asset.bird_ownership_history(bird_id,user_id) VALUES(bird1,eo),(bird1,owner2);
    INSERT INTO asset.gantangan(arena_id,number,baris,kolom) VALUES(arena1,1,1,1) RETURNING id INTO gantangan1;
    INSERT INTO asset.gantangan(arena_id,number,baris,kolom) VALUES(arena1,2,1,2) RETURNING id INTO gantangan2;
    INSERT INTO asset.gantangan(arena_id,number,baris,kolom) VALUES(arena2,1,1,1) RETURNING id INTO gantangan3;
    INSERT INTO competition.events(arena_id,owner_id,title,event_date,status)
    VALUES(arena1,eo,'Verification event',CURRENT_DATE,'ACTIVE') RETURNING id INTO event1;
    INSERT INTO competition.rounds(arena_id,event_id,bird_species_id,round_number,title,status)
    VALUES(arena1,event1,species,1,'Verification round 1','JUDGING') RETURNING id INTO round1;
    INSERT INTO competition.rounds(arena_id,event_id,bird_species_id,round_number,title)
    VALUES(arena1,event1,species,2,'Verification round 2') RETURNING id INTO round2;
    IF (SELECT category_id IS NOT NULL FROM cloud_read.v_rounds WHERE id=round1) THEN RAISE EXCEPTION 'Optional category lost'; END IF;
    IF (SELECT round_count FROM cloud_read.v_events WHERE id=event1) <> 2 THEN RAISE EXCEPTION 'Event rows/count duplicated'; END IF;
    IF (SELECT held_event_count FROM cloud_read.v_eo_arena_history WHERE owner_id=eo AND arena_id=arena1) <> 1 THEN RAISE EXCEPTION 'EO/arena history failed'; END IF;

    BEGIN
        INSERT INTO competition.rounds(arena_id,event_id,bird_species_id,round_number,title)
        VALUES(arena2,event1,species,1,'Wrong arena');
        RAISE EXCEPTION 'Cross-arena event/round accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL; END;
    INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method,payment_status,verified_at,status)
    VALUES(registrant,arena1,round1,bird1,gantangan1,50000,'CASH','VERIFIED',CURRENT_TIMESTAMP,'CONFIRMED') RETURNING id INTO ticket1;
    INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method,payment_status,verified_at,status)
    VALUES(registrant,arena1,round1,bird2,gantangan2,75000,'TRANSFER','VERIFIED',CURRENT_TIMESTAMP,'CONFIRMED') RETURNING id INTO ticket2;
    UPDATE competition.registrations SET payment_proof=decode('89504e47','hex'),payment_proof_mime='image/png' WHERE id=ticket2;
    IF NOT (SELECT has_payment_proof FROM cloud_read.v_registrations WHERE id=ticket2) THEN RAISE EXCEPTION 'Proof flag failed'; END IF;
    BEGIN
        UPDATE competition.registrations SET entry_fee='NaN'::numeric WHERE id=ticket1;
        RAISE EXCEPTION 'Non-finite payment amount accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;
    -- Same bird and gantangan may participate again in an independent round.
    INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method)
    VALUES(registrant,arena1,round2,bird1,gantangan1,50000,'CASH');
    BEGIN
        INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method)
        VALUES(registrant,arena1,round1,bird1,gantangan2,50000,'CASH');
        RAISE EXCEPTION 'Duplicate bird in round accepted';
    EXCEPTION WHEN unique_violation THEN NULL; END;
    BEGIN
        INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method,payment_status,verified_at,status)
        VALUES(registrant,arena1,round1,bird3,gantangan1,50000,'CASH','VERIFIED',CURRENT_TIMESTAMP,'CONFIRMED');
        RAISE EXCEPTION 'Occupied confirmed gantangan accepted';
    EXCEPTION WHEN unique_violation THEN NULL; END;
    BEGIN
        UPDATE competition.registrations SET gantangan_id=gantangan3 WHERE id=ticket1;
        RAISE EXCEPTION 'Foreign arena gantangan accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL; END;
    BEGIN
        UPDATE competition.registrations SET participant_status='DISQUALIFIED' WHERE id=ticket1;
        RAISE EXCEPTION 'Disqualification without reason/time accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;

    INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES(arena1,'Verification criterion 1') RETURNING id INTO criterion1;
    INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES(arena1,'Verification criterion 2') RETURNING id INTO criterion2;
    INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES(arena2,'Verification foreign criterion') RETURNING id INTO foreign_criterion;
    INSERT INTO judging.round_judges(round_id,judge_user_id) VALUES(round1,judge1),(round1,judge2);
    SELECT * INTO result FROM cloud_read.v_round_score_summary WHERE registration_id=ticket1;
    IF result.score_count<>0 OR result.average_score IS NOT NULL OR result.expected_score_count<>4 OR result.is_complete THEN
        RAISE EXCEPTION 'Empty score summary is incorrect';
    END IF;
    INSERT INTO judging.round_scores(arena_id,round_id,registration_id,user_id,criteria_id,score_value)
    VALUES(arena1,round1,ticket1,judge1,criterion1,8.5) RETURNING id INTO first_score;
    INSERT INTO judging.round_scores(arena_id,round_id,registration_id,user_id,criteria_id,score_value) VALUES
    (arena1,round1,ticket1,judge1,criterion2,9),
    (arena1,round1,ticket1,judge2,criterion1,7.5),
    (arena1,round1,ticket1,judge2,criterion2,8);
    SELECT * INTO result FROM cloud_read.v_round_score_summary WHERE registration_id=ticket1;
    IF result.score_count<>4 OR result.average_score<>8.25 OR NOT result.is_complete THEN RAISE EXCEPTION 'Decimal average/completeness failed'; END IF;
    BEGIN
        INSERT INTO judging.round_scores(arena_id,round_id,registration_id,user_id,criteria_id,score_value)
        VALUES(arena1,round1,ticket1,registrant,criterion1,8);
        RAISE EXCEPTION 'Unassigned judge accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL; END;
    BEGIN
        UPDATE judging.round_scores SET criteria_id=foreign_criterion WHERE id=first_score;
        RAISE EXCEPTION 'Cross-arena criterion accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL; END;
    BEGIN
        UPDATE judging.round_scores SET score_value=10.1 WHERE id=first_score;
        RAISE EXCEPTION 'Out-of-range score accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;
    REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
    UPDATE judging.round_scores SET score_value=10,updated_at=CURRENT_TIMESTAMP WHERE id=first_score;
    IF (SELECT average_score FROM cloud_read.v_round_score_summary WHERE registration_id=ticket1)<>8.625 THEN RAISE EXCEPTION 'Score overwrite did not update live average'; END IF;
    IF (SELECT average_score FROM cloud_read.mv_round_score_summary WHERE registration_id=ticket1)<>8.25 THEN RAISE EXCEPTION 'Snapshot unexpectedly changed without refresh'; END IF;
    REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
    IF (SELECT average_score FROM cloud_read.mv_round_score_summary WHERE registration_id=ticket1)<>8.625 THEN RAISE EXCEPTION 'Concurrent refresh failed'; END IF;

    INSERT INTO asset.bird_award_certificate_history(bird_id,registration_id,award_name,rank,awarded_at)
    VALUES(bird1,ticket1,'Verification internal award',1,CURRENT_DATE);
    INSERT INTO asset.bird_award_certificate_history(bird_id,award_name,event_name,awarded_at)
    VALUES(bird1,'Verification external award','External event',CURRENT_DATE);
    IF (SELECT count(*) FROM cloud_read.v_bird_award_certificate_history WHERE bird_id=bird1)<>2 THEN RAISE EXCEPTION 'Optional internal/external award relation failed'; END IF;
    BEGIN
        INSERT INTO asset.bird_award_certificate_history(bird_id,registration_id,award_name,awarded_at)
        VALUES(bird2,ticket1,'Wrong bird',CURRENT_DATE);
        RAISE EXCEPTION 'Wrong bird on internal award accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL; END;
    BEGIN
        INSERT INTO asset.bird_award_certificate_history(bird_id,award_name,awarded_at) VALUES(bird1,'Missing event',CURRENT_DATE);
        RAISE EXCEPTION 'External award without event accepted';
    EXCEPTION WHEN check_violation THEN NULL; END;
    INSERT INTO media.round_recordings(registration_id,video_url) VALUES(ticket1,'https://storage.example.test/fixture.mp4');
    UPDATE iam.users SET phone='+909009999002' WHERE id=registrant;
    IF (SELECT recipient_phone FROM cloud_read.v_round_recordings WHERE registration_id=ticket1)<>'+909009999002' THEN RAISE EXCEPTION 'Media recipient did not follow account phone'; END IF;
    BEGIN
        INSERT INTO media.round_recordings(registration_id) VALUES(ticket1);
        RAISE EXCEPTION 'Duplicate recording accepted';
    EXCEPTION WHEN unique_violation THEN NULL; END;
    -- Disqualification keeps scores and payment but excludes the ticket from recap.
    UPDATE competition.registrations SET participant_status='DISQUALIFIED',disqualification_reason='Verification only',disqualified_at=CURRENT_TIMESTAMP WHERE id=ticket1;
    IF EXISTS(SELECT FROM cloud_read.v_round_score_summary WHERE registration_id=ticket1) OR
       (SELECT count(*) FROM judging.round_scores WHERE registration_id=ticket1)<>4 OR
       (SELECT payment_status FROM competition.registrations WHERE id=ticket1)<>'VERIFIED' THEN
        RAISE EXCEPTION 'Disqualification damaged scores/payment or remained eligible';
    END IF;
    REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
    REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_arena_dashboard;
    IF (SELECT confirmed_participant_count FROM cloud_read.mv_arena_dashboard WHERE arena_id=arena1)<>1 THEN RAISE EXCEPTION 'Dashboard eligibility failed'; END IF;
    IF has_table_privilege('kicau_cloud_reader','cloud_read.v_round_scores','SELECT') OR
       has_table_privilege('kicau_cloud_reader','cloud_read.v_users','SELECT') THEN RAISE EXCEPTION 'Reader can read private IAM/scoring'; END IF;
    IF EXISTS(SELECT FROM information_schema.columns WHERE table_schema='cloud_read' AND column_name IN ('password_hash','provider_user_id','payment_proof','video_url','audio_url','certificate_url')) THEN
        RAISE EXCEPTION 'Sensitive raw data leaked into read layer';
    END IF;
    RAISE NOTICE 'PASS: role-limited CRUD, optional joins, arena/roster FKs, ticket uniqueness, decimal revisions, snapshots, disqualification, awards, media recipient and read grants';
END;
$verify$;
ROLLBACK;
