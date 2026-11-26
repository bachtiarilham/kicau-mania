-- Additional application contracts for schema 010. No DDL or legacy aliases.

-- name: SchemaReady
SELECT jsonb_build_object('ready',true) WHERE NOT EXISTS(SELECT FROM unnest(ARRAY['migration_ids.legacy','iam.user_master_history','asset.bird_master_history','cloud_read.v_users','cloud_read.v_birds','cloud_read.v_registrations','cloud_read.v_round_score_summary','media.ingest_engines','media.ingest_sessions','media.ingest_mappings','media.recording_assets']) x(name) WHERE to_regclass(x.name) IS NULL) AND (SELECT count(*) FROM iam.roles WHERE name IN ('ADMIN','OPERATOR','JURI','PESERTA'))=4;

-- name: ActorProfile
-- params: user_id bigint
SELECT to_jsonb(v)||jsonb_build_object('roles',coalesce((SELECT jsonb_agg(r.name ORDER BY r.name) FROM iam.user_roles u JOIN iam.roles r ON r.id=u.role_id WHERE u.user_id=v.id),'[]'::jsonb),'arena_roles',coalesce((SELECT jsonb_agg(jsonb_build_object('arena_id',a.arena_id,'role',r.name) ORDER BY a.arena_id,r.name) FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=v.id),'[]'::jsonb)) FROM cloud_read.v_users v WHERE id=$1::text::bigint AND deleted_at IS NULL AND is_active;

-- name: ActorArenas
-- params: actor_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE ($2::text::bigint IS NULL OR v.id>$2) AND (EXISTS(SELECT FROM iam.arena_user_roles a WHERE a.arena_id=v.id AND a.user_id=$1::text::bigint) OR EXISTS(SELECT FROM competition.registrations t WHERE t.arena_id=v.id AND t.user_id=$1::text::bigint)) ORDER BY v.id LIMIT $3;

-- name: AccountList
-- params: cursor bigint nullable, limit int, is_eo boolean nullable, role text nullable, search text nullable
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE deleted_at IS NULL AND ($1::text::bigint IS NULL OR id>$1) AND ($3::boolean IS NULL OR is_eo=$3) AND ($4::text IS NULL OR EXISTS(SELECT FROM iam.user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=v.id AND r.name=$4) OR EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=v.id AND r.name=$4)) AND ($5::text IS NULL OR strpos(lower(concat_ws(' ',v.name,v.phone,v.email)),lower($5))>0) ORDER BY id LIMIT $2;

-- name: ActorAssignments
-- params: arena_id bigint
SELECT to_jsonb(v)||jsonb_build_object('role',r.name,'name',u.name,'email',u.email) FROM cloud_read.v_arena_user_roles v JOIN iam.roles r ON r.id=v.role_id JOIN iam.users u ON u.id=v.user_id WHERE v.arena_id=$1::text::bigint ORDER BY r.name,u.name,v.id;

-- name: AssignmentCount
-- params: arena_id bigint
SELECT jsonb_build_object('count',count(*)) FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id JOIN iam.users u ON u.id=a.user_id WHERE a.arena_id=$1::text::bigint AND r.name='ADMIN' AND u.is_active AND u.deleted_at IS NULL;

-- name: PrivilegedAccount
-- params: target_id bigint, actor_id bigint
SELECT jsonb_build_object('privileged',EXISTS(SELECT FROM iam.user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::text::bigint AND r.name IN ('ADMIN','OPERATOR','JURI')) OR EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::text::bigint AND r.name IN ('ADMIN','OPERATOR','JURI')),'manageable',NOT EXISTS(SELECT FROM iam.user_roles g JOIN iam.roles z ON z.id=g.role_id WHERE g.user_id=$1::text::bigint AND z.name='ADMIN') AND NOT EXISTS(SELECT FROM iam.arena_user_roles x JOIN iam.roles z ON z.id=x.role_id WHERE x.user_id=$1::text::bigint AND z.name IN ('ADMIN','OPERATOR','JURI') AND NOT EXISTS(SELECT FROM iam.arena_user_roles y JOIN iam.roles ar ON ar.id=y.role_id WHERE y.user_id=$2::text::bigint AND y.arena_id=x.arena_id AND ar.name='ADMIN')) AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id JOIN iam.arena_user_roles t ON t.arena_id=a.arena_id WHERE a.user_id=$2::text::bigint AND r.name='ADMIN' AND t.user_id=$1::text::bigint));

-- name: AccountAdminScopes
-- params: user_id bigint
SELECT jsonb_build_object('id',a.arena_id) FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::text::bigint AND r.name='ADMIN' ORDER BY a.arena_id;

-- name: RegistrationUpdate
-- params: arena_id bigint, registration_id bigint, user_id bigint, round_id bigint, bird_id bigint, gantangan_id bigint, entry_fee numeric, payment_method text, registration_number text nullable, registration_date date nullable, participant_name text nullable, participant_phone text nullable, owner_name text nullable, owner_phone text nullable
UPDATE competition.registrations SET user_id=$3::text::bigint,round_id=$4::text::bigint,bird_id=$5::text::bigint,gantangan_id=$6::text::bigint,entry_fee=$7::numeric,payment_method=$8,registration_number=$9,registration_date=coalesce($10::date,CURRENT_DATE),participant_name=$11,participant_phone=$12,owner_name=$13,owner_phone=$14,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND status='PENDING_PAYMENT' AND payment_status<>'VERIFIED' RETURNING jsonb_build_object('id',id);

-- name: ParticipantRequalify
-- params: arena_id bigint, registration_id bigint
UPDATE competition.registrations SET participant_status='ACTIVE',disqualification_reason=NULL,disqualified_at=NULL,updated_at=CURRENT_TIMESTAMP WHERE arena_id=$1::text::bigint AND id=$2::text::bigint AND participant_status='DISQUALIFIED' RETURNING jsonb_build_object('id',id);

-- name: RoundRegistrations
-- params: arena_id bigint, round_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint ORDER BY id;

-- name: JudgeAccountForArena
-- params: user_id bigint, arena_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE v.id=$1::text::bigint AND v.is_active AND v.deleted_at IS NULL AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=v.id AND a.arena_id=$2::text::bigint AND r.name='JURI');

-- name: CriteriaForArena
-- params: arena_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::text::bigint ORDER BY id;

-- name: BirdHasRegistrations
-- params: bird_id bigint
SELECT jsonb_build_object('used',EXISTS(SELECT FROM competition.registrations WHERE bird_id=$1::text::bigint));

-- name: ResetEmailVerification
-- params: user_id bigint
UPDATE iam.users SET email_verified_at=NULL WHERE id=$1::text::bigint;

-- name: AccountCredentialByID
-- params: user_id bigint
SELECT jsonb_build_object('id',id,'password_hash',password_hash) FROM iam.users WHERE id=$1::text::bigint AND deleted_at IS NULL AND is_active;

-- name: BootstrapAdminExists
SELECT jsonb_build_object('exists',EXISTS(SELECT FROM iam.user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE r.name='ADMIN'));

-- name: SessionRevokeByID
-- params: session_id bigint
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE id=$1::text::bigint AND revoked_at IS NULL;

-- name: ParticipantRegistrations
-- params: user_id bigint, cursor bigint nullable, limit int, arena_id bigint nullable
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE user_id=$1::text::bigint AND ($2::text::bigint IS NULL OR v.id>$2) AND ($4::text::bigint IS NULL OR arena_id=$4) ORDER BY v.id LIMIT $3;

-- name: AwardRaw
-- params: bird_id bigint, award_id bigint
SELECT jsonb_build_object('id',id,'bird_id',bird_id,'registration_id',registration_id,'certificate_url',certificate_url) FROM asset.bird_award_certificate_history WHERE bird_id=$1::text::bigint AND id=$2::text::bigint;

-- name: RoundFilteredList
-- params: arena_id bigint, event_id bigint nullable, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR event_id=$2) AND ($3::text::bigint IS NULL OR id>$3) ORDER BY id LIMIT $4;

-- name: RegistrationFilteredList
-- params: arena_id bigint, round_id bigint nullable, event_id bigint nullable, cursor bigint nullable, limit int, bird_id bigint nullable
SELECT to_jsonb(v) FROM cloud_read.v_registrations v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR round_id=$2) AND ($3::text::bigint IS NULL OR event_id=$3) AND ($4::text::bigint IS NULL OR id>$4) AND ($6::text::bigint IS NULL OR bird_id=$6) ORDER BY id LIMIT $5;

-- name: JudgeAssignmentActive
-- params: arena_id bigint, round_id bigint, user_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_round_judges v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint AND judge_user_id=$3::text::bigint AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$3::text::bigint AND a.arena_id=$1::text::bigint AND r.name='JURI');

-- name: ClearArenaAssignments
-- params: arena_id bigint
DELETE FROM iam.arena_user_roles WHERE arena_id=$1::text::bigint;

-- name: AccountCreate
-- params: user_id bigint, name text, phone text nullable, email text nullable, password_hash text nullable, is_eo boolean, is_active boolean, nickname text nullable, address text nullable, position text nullable
INSERT INTO iam.users(id,name,phone,email,password_hash,is_eo,is_active,nickname,address,position) VALUES($1::text::bigint,$2,$3,lower(btrim($4)),$5,$6,$7,$8,$9,$10) RETURNING jsonb_build_object('id',id,'version',version);

-- name: BootstrapLock
SELECT pg_advisory_xact_lock(753125);

-- name: ArenaJudgeLookup
-- params: arena_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE v.deleted_at IS NULL AND v.is_active AND ($2::text::bigint IS NULL OR v.id>$2) AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=v.id AND a.arena_id=$1::text::bigint AND r.name='JURI') ORDER BY v.id LIMIT $3;

-- name: UserHasOpenJudging
-- params: user_id bigint, arena_id bigint nullable
SELECT jsonb_build_object('assigned',EXISTS(SELECT FROM judging.round_judges j JOIN competition.rounds r ON r.id=j.round_id WHERE j.judge_user_id=$1::text::bigint AND r.status IN ('RUNNING','JUDGING') AND ($2::text::bigint IS NULL OR r.arena_id=$2)));

-- name: BirdFilteredList
-- params: owner_id bigint nullable, cursor bigint nullable, limit int, search text nullable
SELECT to_jsonb(v) FROM cloud_read.v_birds v WHERE deleted_at IS NULL AND ($1::text::bigint IS NULL OR owner_id=$1) AND ($2::text::bigint IS NULL OR id>$2) AND ($4::text IS NULL OR strpos(lower(concat_ws(' ',v.name,v.bird_code,v.owner_name,v.bird_species_name)),lower($4))>0) ORDER BY id LIMIT $3;
