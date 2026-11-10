-- Named statements; placeholders are bound by pgx. No PREPARE execution required.

-- name: SchemaReady
SELECT jsonb_build_object('ready',true) WHERE NOT EXISTS(SELECT FROM unnest(ARRAY['cloud_read.v_user_auth_providers','cloud_read.v_users','cloud_read.v_roles','cloud_read.v_user_roles','cloud_read.v_arena_user_roles','cloud_read.v_user_sessions','cloud_read.v_arenas','cloud_read.v_gantangan','cloud_read.v_engines','cloud_read.v_devices','cloud_read.v_owners','cloud_read.v_birds','cloud_read.v_bird_owner_transfers','cloud_read.v_events','cloud_read.v_categories','cloud_read.v_rounds','cloud_read.v_registrations','cloud_read.v_registration_items','cloud_read.v_registration_payments','cloud_read.v_score_criteria','cloud_read.v_round_judges','cloud_read.v_round_scores','cloud_read.v_score_revisions','cloud_read.v_participant_assessments','cloud_read.v_round_recordings','cloud_read.v_round_score_summary','cloud_read.mv_round_score_summary','cloud_read.mv_arena_dashboard','iam.account_tokens','judging.participant_assessments','judging.score_operations','cloud_read.refresh_status']) n(name) WHERE to_regclass(n.name) IS NULL) AND (SELECT count(*) FROM iam.roles WHERE name IN ('superadmin','admin_arena','operator','juri','peserta','penjoki'))=6 AND EXISTS(SELECT FROM information_schema.columns WHERE table_schema='iam' AND table_name='user_sessions' AND column_name='access_token_hash');

-- name: GlobalRole
SELECT jsonb_build_object('allowed',exists(SELECT FROM iam.user_roles ur JOIN iam.roles r ON r.id=ur.role_id WHERE ur.user_id=$1::uuid AND r.name=ANY($2::text[])));

-- name: ArenaRole
SELECT jsonb_build_object('allowed',exists(SELECT FROM iam.arena_user_roles ur JOIN iam.roles r ON r.id=ur.role_id WHERE ur.user_id=$1::uuid AND ur.arena_id=$2::uuid AND r.name=ANY($3::text[])));

-- name: UserArenas
SELECT to_jsonb(v) FROM cloud_read.v_arenas v WHERE $2::boolean OR EXISTS(SELECT FROM iam.arena_user_roles a WHERE a.user_id=$1::uuid AND a.arena_id=v.id) ORDER BY v.name,v.id LIMIT $3;

-- name: UserLookup
SELECT jsonb_build_object('id',id,'name',name) FROM cloud_read.v_users WHERE lower(email)=lower($1);

-- name: AssignGlobalRole
INSERT INTO iam.user_roles(user_id,role_id) SELECT $1::uuid,id FROM iam.roles WHERE name=$2 ON CONFLICT DO NOTHING;

-- name: AssignArenaRole
INSERT INTO iam.arena_user_roles(arena_id,user_id,role_id) SELECT $1::uuid,$2::uuid,id FROM iam.roles WHERE name=$3 ON CONFLICT DO NOTHING;

-- name: RevokeGlobalRole
DELETE FROM iam.user_roles WHERE id=$1::uuid;

-- name: RevokeArenaRole
DELETE FROM iam.arena_user_roles WHERE id=$1::uuid AND arena_id=$2::uuid;

-- name: UserProfile
SELECT to_jsonb(v)||jsonb_build_object('auth_providers',coalesce((SELECT jsonb_agg(p.provider ORDER BY p.provider) FROM cloud_read.v_user_auth_providers p WHERE p.user_id=v.id),'[]'::jsonb),'roles',coalesce((SELECT jsonb_agg(role.name ORDER BY role.name) FROM iam.user_roles ur JOIN iam.roles role ON role.id=ur.role_id WHERE ur.user_id=v.id),'[]'::jsonb),'arena_roles',coalesce((SELECT jsonb_agg(jsonb_build_object('arena_id',ar.arena_id,'role',role.name) ORDER BY ar.arena_id,role.name) FROM iam.arena_user_roles ar JOIN iam.roles role ON role.id=ar.role_id WHERE ar.user_id=v.id),'[]'::jsonb)) FROM cloud_read.v_users v WHERE id=$1::uuid;

-- name: PatchProfile
UPDATE iam.users SET name=CASE WHEN $2::jsonb?'name' THEN $2->>'name' ELSE name END,phone=CASE WHEN $2::jsonb?'phone' THEN $2->>'phone' ELSE phone END,updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: SetUserActive
UPDATE iam.users SET is_active=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: ListUsers
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE id::text>$1 ORDER BY id LIMIT $2;

-- name: ListRoles
SELECT to_jsonb(v) FROM cloud_read.v_roles v ORDER BY name,id;

-- name: ListAssignments
SELECT to_jsonb(v)||jsonb_build_object('role',r.name) FROM cloud_read.v_arena_user_roles v JOIN cloud_read.v_roles r ON r.id=v.role_id WHERE arena_id=$1::uuid ORDER BY v.id;

-- name: RemoveGlobalRole
DELETE FROM iam.user_roles ur USING iam.roles r WHERE ur.user_id=$1::uuid AND ur.role_id=r.id AND r.name=$2 RETURNING jsonb_build_object('id',ur.id);
