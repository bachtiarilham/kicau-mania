-- Named statements; placeholders are bound by pgx. No PREPARE execution required.

-- name: EntityEventsGet
SELECT to_jsonb(v) FROM cloud_read.v_events v WHERE id=$1::uuid;

-- name: EntityEventsList
SELECT to_jsonb(v) FROM cloud_read.v_events v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityEventsCreate
INSERT INTO competition.events(arena_id,title,date,location,status) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'title')::text,($1::jsonb->>'date')::date,($1::jsonb->>'location')::text,coalesce(($1::jsonb->>'status')::text,'scheduled')) RETURNING jsonb_build_object('id',id);

-- name: EntityEventsUpdate
UPDATE competition.events SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,title=CASE WHEN $2::jsonb?'title' THEN ($2->>'title')::text ELSE title END,date=CASE WHEN $2::jsonb?'date' THEN ($2->>'date')::date ELSE date END,location=CASE WHEN $2::jsonb?'location' THEN ($2->>'location')::text ELSE location END,status=CASE WHEN $2::jsonb?'status' THEN ($2->>'status')::text ELSE status END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityEventsDelete
DELETE FROM competition.events WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityCategoriesGet
SELECT to_jsonb(v) FROM cloud_read.v_categories v WHERE id=$1::uuid;

-- name: EntityCategoriesList
SELECT to_jsonb(v) FROM cloud_read.v_categories v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityCategoriesCreate
INSERT INTO competition.categories(arena_id,code,name) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'code')::text,($1::jsonb->>'name')::text) RETURNING jsonb_build_object('id',id);

-- name: EntityCategoriesUpdate
UPDATE competition.categories SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,code=CASE WHEN $2::jsonb?'code' THEN ($2->>'code')::text ELSE code END,name=CASE WHEN $2::jsonb?'name' THEN ($2->>'name')::text ELSE name END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityCategoriesDelete
DELETE FROM competition.categories WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityRoundsGet
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE id=$1::uuid;

-- name: EntityRoundsList
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityRoundsCreate
INSERT INTO competition.rounds(arena_id,event_id,category_id,round_number,bird_type,entry_fee,session_duration_minutes,judging_duration_minutes) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'event_id')::uuid,($1::jsonb->>'category_id')::uuid,($1::jsonb->>'round_number')::integer,($1::jsonb->>'bird_type')::text,coalesce(($1::jsonb->>'entry_fee')::numeric,0),coalesce(($1::jsonb->>'session_duration_minutes')::integer,10),coalesce(($1::jsonb->>'judging_duration_minutes')::integer,15)) RETURNING jsonb_build_object('id',id);

-- name: EntityRoundsUpdate
UPDATE competition.rounds SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,event_id=CASE WHEN $2::jsonb?'event_id' THEN ($2->>'event_id')::uuid ELSE event_id END,category_id=CASE WHEN $2::jsonb?'category_id' THEN ($2->>'category_id')::uuid ELSE category_id END,round_number=CASE WHEN $2::jsonb?'round_number' THEN ($2->>'round_number')::integer ELSE round_number END,bird_type=CASE WHEN $2::jsonb?'bird_type' THEN ($2->>'bird_type')::text ELSE bird_type END,entry_fee=CASE WHEN $2::jsonb?'entry_fee' THEN ($2->>'entry_fee')::numeric ELSE entry_fee END,session_duration_minutes=CASE WHEN $2::jsonb?'session_duration_minutes' THEN ($2->>'session_duration_minutes')::integer ELSE session_duration_minutes END,judging_duration_minutes=CASE WHEN $2::jsonb?'judging_duration_minutes' THEN ($2->>'judging_duration_minutes')::integer ELSE judging_duration_minutes END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityRoundsDelete
DELETE FROM competition.rounds WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: LockArena
SELECT to_jsonb(a) FROM asset.arenas a WHERE id=$1::uuid FOR UPDATE;

-- name: LockRound
SELECT to_jsonb(r) FROM competition.rounds r WHERE id=$1::uuid FOR UPDATE;

-- name: LockCriteria
UPDATE asset.arenas SET criteria_locked_at=coalesce(criteria_locked_at,CURRENT_TIMESTAMP) WHERE id=$1::uuid;

-- name: SetRoundPhase
UPDATE competition.rounds SET phase=$2::varchar,interrupted_from_phase=$3,start_time=CASE WHEN $2::varchar='RUNNING' THEN coalesce(start_time,CURRENT_TIMESTAMP) ELSE start_time END,end_time=CASE WHEN $2::varchar IN ('CLOSED','CANCELLED') AND start_time IS NOT NULL THEN CURRENT_TIMESTAMP ELSE end_time END,winner_registration_item_id=$4::uuid,winner_confirmed_by_user_id=$5::uuid,winner_confirmed_at=CASE WHEN $2::varchar='CLOSED' THEN CURRENT_TIMESTAMP END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: RoundItems
SELECT to_jsonb(v) FROM cloud_read.v_registration_items v WHERE round_id=$1::uuid AND status='CONFIRMED' ORDER BY gantangan_number,id;

-- name: RoundJudges
SELECT to_jsonb(v) FROM cloud_read.v_round_judges v WHERE round_id=$1::uuid ORDER BY id;

-- name: ArenaCriteria
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::uuid ORDER BY id;

-- name: AssignJudge
INSERT INTO judging.round_judges(arena_id,round_id,judge_user_id) VALUES($1::uuid,$2::uuid,$3::uuid) RETURNING jsonb_build_object('id',id);

-- name: RemoveJudge
DELETE FROM judging.round_judges WHERE id=$1::uuid AND round_id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: LiveRecap
SELECT to_jsonb(v) FROM cloud_read.v_round_score_summary v WHERE round_id=$1::uuid ORDER BY registration_item_id;

-- name: RoundAllItems
SELECT to_jsonb(v) FROM cloud_read.v_registration_items v WHERE round_id=$1::uuid ORDER BY id;

-- name: CreateRegistration
INSERT INTO competition.registrations(arena_id,registered_by_user_id,created_by_user_id,registration_source,registrant_name,registrant_phone,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,'PENDING_PAYMENT') RETURNING jsonb_build_object('id',id);

-- name: InsertRegistrationItem
INSERT INTO competition.registration_items(arena_id,registration_id,round_id,bird_id,entry_fee_snapshot) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::numeric);

-- name: SetRegistrationTotal
UPDATE competition.registrations SET total_amount=$2::numeric,registrant_name=$3,registrant_phone=$4 WHERE id=$1::uuid;

-- name: RegistrationGet
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE id=$1::uuid;

-- name: RegistrationLock
SELECT to_jsonb(v) FROM competition.registrations v WHERE id=$1::uuid FOR UPDATE;

-- name: RegistrationList
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) AND ($4::boolean OR registered_by_user_id=$3::uuid OR created_by_user_id=$3::uuid) ORDER BY id LIMIT $5;

-- name: RegistrationItems
SELECT to_jsonb(v) FROM cloud_read.v_registration_items v WHERE registration_id=$1::uuid ORDER BY id;

-- name: RegistrationPayments
SELECT to_jsonb(v) FROM cloud_read.v_registration_payments v WHERE registration_id=$1::uuid ORDER BY created_at,id;

-- name: DeleteRegistrationItems
DELETE FROM competition.registration_items WHERE registration_id=$1::uuid AND status='PENDING_PAYMENT';

-- name: CancelRegistration
UPDATE competition.registrations SET status='CANCELLED' WHERE id=$1::uuid;

-- name: CancelRegistrationItems
UPDATE competition.registration_items SET status='CANCELLED' WHERE registration_id=$1::uuid AND status IN ('DRAFT','PENDING_PAYMENT');

-- name: CancelPendingPayments
UPDATE competition.registration_payments SET status='REJECTED' WHERE registration_id=$1::uuid AND status='PENDING';

-- name: PaymentCreate
INSERT INTO competition.registration_payments(arena_id,registration_id,amount,method,proof_reference) VALUES($1::uuid,$2::uuid,$3::numeric,$4,$5) RETURNING jsonb_build_object('id',id);

-- name: PaymentGet
SELECT to_jsonb(v) FROM cloud_read.v_registration_payments v WHERE id=$1::uuid;

-- name: PaymentLock
SELECT to_jsonb(v) FROM competition.registration_payments v WHERE id=$1::uuid FOR UPDATE;

-- name: PaymentVerify
UPDATE competition.registration_payments SET status='VERIFIED',verified_by_user_id=$2::uuid,verified_at=CURRENT_TIMESTAMP WHERE id=$1::uuid AND status='PENDING' RETURNING jsonb_build_object('id',id);

-- name: PaymentReject
UPDATE competition.registration_payments SET status='REJECTED' WHERE id=$1::uuid AND status='PENDING' RETURNING jsonb_build_object('id',id);

-- name: ConfirmItem
UPDATE competition.registration_items SET gantangan_id=$3::uuid,status='CONFIRMED' WHERE id=$1::uuid AND registration_id=$2::uuid AND status='PENDING_PAYMENT' RETURNING jsonb_build_object('id',id);

-- name: ConfirmRegistration
UPDATE competition.registrations SET status='CONFIRMED' WHERE id=$1::uuid;
