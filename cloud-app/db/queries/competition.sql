-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: EventList
-- params: arena_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_events v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: EventGet
-- params: arena_id bigint, id bigint
SELECT to_jsonb(v) FROM cloud_read.v_events v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: EventCreate
-- params: arena_id bigint, owner_id bigint, title text, description text, event_date date, status text, location text nullable, grand_prize text nullable, pic_name text nullable, min_participants int nullable, max_participants int nullable
INSERT INTO competition.events(arena_id,owner_id,title,description,event_date,status,location,grand_prize,pic_name,min_participants,max_participants) VALUES($1::text::bigint,$2::text::bigint,$3::text,$4::text,$5::date,$6::text,$7,$8,$9,coalesce($10::int,0),coalesce($11::int,0)) RETURNING jsonb_build_object('id',id);

-- name: EventUpdate
-- params: arena_id bigint, id bigint, patch jsonb
UPDATE competition.events SET owner_id=CASE WHEN $3::jsonb ? 'owner_id' THEN ($3::jsonb->>'owner_id')::uuid ELSE owner_id END,
 title=CASE WHEN $3::jsonb ? 'title' THEN ($3::jsonb->>'title')::text ELSE title END,
 description=CASE WHEN $3::jsonb ? 'description' THEN ($3::jsonb->>'description')::text ELSE description END,
 event_date=CASE WHEN $3::jsonb ? 'event_date' THEN ($3::jsonb->>'event_date')::date ELSE event_date END,
 status=CASE WHEN $3::jsonb ? 'status' THEN ($3::jsonb->>'status')::text ELSE status END,
 location=CASE WHEN $3::jsonb ? 'location' THEN ($3::jsonb->>'location')::text ELSE location END,
 grand_prize=CASE WHEN $3::jsonb ? 'grand_prize' THEN ($3::jsonb->>'grand_prize')::text ELSE grand_prize END,
 pic_name=CASE WHEN $3::jsonb ? 'pic_name' THEN ($3::jsonb->>'pic_name')::text ELSE pic_name END,
 min_participants=CASE WHEN $3::jsonb ? 'min_participants' THEN ($3::jsonb->>'min_participants')::int ELSE min_participants END,
 max_participants=CASE WHEN $3::jsonb ? 'max_participants' THEN ($3::jsonb->>'max_participants')::int ELSE max_participants END,
 updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: EventDelete
-- params: arena_id bigint, id bigint
DELETE FROM competition.events WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: CategoryList
-- params: arena_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_categories v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: CategoryGet
-- params: arena_id bigint, id bigint
SELECT to_jsonb(v) FROM cloud_read.v_categories v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: CategoryCreate
-- params: arena_id bigint, code text, name text
INSERT INTO competition.categories(arena_id,code,name) VALUES($1::text::bigint,$2::text,$3::text) RETURNING jsonb_build_object('id',id);

-- name: CategoryUpdate
-- params: arena_id bigint, id bigint, patch jsonb
UPDATE competition.categories SET code=CASE WHEN $3::jsonb ? 'code' THEN ($3::jsonb->>'code')::text ELSE code END,
 name=CASE WHEN $3::jsonb ? 'name' THEN ($3::jsonb->>'name')::text ELSE name END WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: CategoryDelete
-- params: arena_id bigint, id bigint
DELETE FROM competition.categories WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: RoundList
-- params: arena_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: RoundGet
-- params: arena_id bigint, id bigint
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: RoundCreate
-- params: arena_id bigint, event_id bigint, category_id bigint, bird_species_id bigint, round_number int, title text, start_time text nullable, end_time text nullable, coordinator_name text nullable, head_judge_name text nullable, field_judge_1 text nullable, field_judge_2 text nullable, field_judge_3 text nullable, field_judge_4 text nullable, monitor_judge_1 text nullable, monitor_judge_2 text nullable, monitor_judge_3 text nullable, monitor_judge_4 text nullable
INSERT INTO competition.rounds(arena_id,event_id,category_id,bird_species_id,round_number,title,start_time,end_time,coordinator_name,head_judge_name,field_judge_1,field_judge_2,field_judge_3,field_judge_4,monitor_judge_1,monitor_judge_2,monitor_judge_3,monitor_judge_4) VALUES($1::text::bigint,$2::text::bigint,$3::text::bigint,$4::text::bigint,$5::int,$6::text,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18) RETURNING jsonb_build_object('id',id);

-- name: RoundUpdate
-- params: arena_id bigint, id bigint, patch jsonb
UPDATE competition.rounds SET event_id=CASE WHEN $3::jsonb ? 'event_id' THEN ($3::jsonb->>'event_id')::uuid ELSE event_id END,
 category_id=CASE WHEN $3::jsonb ? 'category_id' THEN ($3::jsonb->>'category_id')::uuid ELSE category_id END,
 bird_species_id=CASE WHEN $3::jsonb ? 'bird_species_id' THEN ($3::jsonb->>'bird_species_id')::uuid ELSE bird_species_id END,
 round_number=CASE WHEN $3::jsonb ? 'round_number' THEN ($3::jsonb->>'round_number')::int ELSE round_number END,
 title=CASE WHEN $3::jsonb ? 'title' THEN ($3::jsonb->>'title')::text ELSE title END,
 start_time=CASE WHEN $3::jsonb ? 'start_time' THEN ($3::jsonb->>'start_time')::text ELSE start_time END,
 end_time=CASE WHEN $3::jsonb ? 'end_time' THEN ($3::jsonb->>'end_time')::text ELSE end_time END,
 coordinator_name=CASE WHEN $3::jsonb ? 'coordinator_name' THEN ($3::jsonb->>'coordinator_name')::text ELSE coordinator_name END,
 head_judge_name=CASE WHEN $3::jsonb ? 'head_judge_name' THEN ($3::jsonb->>'head_judge_name')::text ELSE head_judge_name END,
 field_judge_1=CASE WHEN $3::jsonb ? 'field_judge_1' THEN ($3::jsonb->>'field_judge_1')::text ELSE field_judge_1 END,
 field_judge_2=CASE WHEN $3::jsonb ? 'field_judge_2' THEN ($3::jsonb->>'field_judge_2')::text ELSE field_judge_2 END,
 field_judge_3=CASE WHEN $3::jsonb ? 'field_judge_3' THEN ($3::jsonb->>'field_judge_3')::text ELSE field_judge_3 END,
 field_judge_4=CASE WHEN $3::jsonb ? 'field_judge_4' THEN ($3::jsonb->>'field_judge_4')::text ELSE field_judge_4 END,
 monitor_judge_1=CASE WHEN $3::jsonb ? 'monitor_judge_1' THEN ($3::jsonb->>'monitor_judge_1')::text ELSE monitor_judge_1 END,
 monitor_judge_2=CASE WHEN $3::jsonb ? 'monitor_judge_2' THEN ($3::jsonb->>'monitor_judge_2')::text ELSE monitor_judge_2 END,
 monitor_judge_3=CASE WHEN $3::jsonb ? 'monitor_judge_3' THEN ($3::jsonb->>'monitor_judge_3')::text ELSE monitor_judge_3 END,
 monitor_judge_4=CASE WHEN $3::jsonb ? 'monitor_judge_4' THEN ($3::jsonb->>'monitor_judge_4')::text ELSE monitor_judge_4 END,
 updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: RoundDelete
-- params: arena_id bigint, id bigint
DELETE FROM competition.rounds WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: ArenaLock
-- params: arena_id bigint
SELECT jsonb_build_object('id',id,'is_active',is_active,'criteria_locked_at',criteria_locked_at) FROM asset.arenas WHERE id=$1::text::bigint FOR UPDATE;

-- name: RoundLock
-- params: arena_id bigint, round_id bigint
SELECT jsonb_build_object('id',id,'arena_id',arena_id,'status',status,'bird_species_id',bird_species_id,'event_id',event_id) FROM competition.rounds WHERE arena_id=$1::text::bigint AND id=$2::text::bigint FOR UPDATE;

-- name: RoundSetStatus
-- params: arena_id bigint, round_id bigint, expected_status text, new_status text
UPDATE competition.rounds SET status=$4,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status=$3 RETURNING jsonb_build_object('id',id,'status',status);

-- name: ArenaLockCriteria
-- params: arena_id bigint
UPDATE asset.arenas SET criteria_locked_at=coalesce(criteria_locked_at,CURRENT_TIMESTAMP) WHERE id=$1::text::bigint;

-- name: RegistrationList
-- params: arena_id bigint, round_id bigint nullable, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR round_id=$2) AND ($3::text::bigint IS NULL OR id>$3) ORDER BY id LIMIT $4::int;

-- name: RegistrationGet
-- params: arena_id bigint, registration_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: RegistrationLock
-- params: arena_id bigint, registration_id bigint
SELECT jsonb_build_object('id',id,'arena_id',arena_id,'round_id',round_id,'bird_id',bird_id,'user_id',user_id,'status',status,'payment_status',payment_status,'participant_status',participant_status) FROM competition.registrations WHERE arena_id=$1::text::bigint AND id=$2::text::bigint FOR UPDATE;

-- name: RegistrationCreate
-- params: registrant_id bigint, arena_id bigint, round_id bigint, bird_id bigint, gantangan_id bigint, entry_fee numeric, payment_method text, registration_number text nullable, registration_date date nullable, participant_name text nullable, participant_phone text nullable, owner_name text nullable, owner_phone text nullable
INSERT INTO competition.registrations(user_id,arena_id,round_id,bird_id,gantangan_id,entry_fee,payment_method,registration_number,registration_date,participant_name,participant_phone,owner_name,owner_phone) SELECT u.id,$2::text::bigint,r.id,b.id,$5::text::bigint,$6::numeric,$7,$8,coalesce($9::date,CURRENT_DATE),$10,$11,$12,$13 FROM iam.users u CROSS JOIN asset.birds b JOIN competition.rounds r ON r.bird_species_id=b.bird_species_id WHERE u.id=$1::text::bigint AND u.deleted_at IS NULL AND u.is_active AND b.id=$4::text::bigint AND b.deleted_at IS NULL AND r.id=$3::text::bigint AND r.arena_id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: RegistrationSetProof
-- params: arena_id bigint, registration_id bigint, image_bytes bytea, image_mime text
UPDATE competition.registrations SET payment_proof=$3::bytea,payment_proof_mime=$4,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status='PENDING_PAYMENT' AND payment_status<>'VERIFIED' RETURNING jsonb_build_object('id',id);

-- name: RegistrationVerifyPayment
-- params: arena_id bigint, registration_id bigint
UPDATE competition.registrations SET payment_status='VERIFIED',verified_at=CURRENT_TIMESTAMP,status='CONFIRMED',updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status='PENDING_PAYMENT' RETURNING jsonb_build_object('id',id);

-- name: RegistrationRejectPayment
-- params: arena_id bigint, registration_id bigint
UPDATE competition.registrations SET payment_status='REJECTED',verified_at=NULL,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status='PENDING_PAYMENT' RETURNING jsonb_build_object('id',id);

-- name: RegistrationCancel
-- params: arena_id bigint, registration_id bigint
UPDATE competition.registrations SET status='CANCELLED',updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status<>'CANCELLED' RETURNING jsonb_build_object('id',id);

-- name: ParticipantDisqualify
-- params: arena_id bigint, registration_id bigint, reason text
UPDATE competition.registrations SET participant_status='DISQUALIFIED',disqualification_reason=$3,disqualified_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND participant_status='ACTIVE' RETURNING jsonb_build_object('id',id);

-- name: RegistrationProofInternal
-- params: arena_id bigint, registration_id bigint
SELECT jsonb_build_object('payment_proof_base64',encode(payment_proof,'base64'),'payment_proof_mime',payment_proof_mime) FROM competition.registrations WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: RoundParticipants
-- params: arena_id bigint, round_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint AND status='CONFIRMED' AND payment_status='VERIFIED' ORDER BY gantangan_number,id;

-- name: EOArenaHistory
-- params: eo_user_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_eo_arena_history v WHERE owner_id=$1::text::bigint ORDER BY arena_id;
