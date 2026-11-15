-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: SetMutationContext
-- params: actor_id uuid, arena_id uuid nullable
SELECT jsonb_build_object('actor_id',set_config('app.actor_id',$1::uuid::text,true),'arena_id',set_config('app.arena_id',coalesce($2::uuid::text,''),true));

-- name: ActorGlobalRoles
-- params: actor_id uuid
SELECT to_jsonb(r) FROM cloud_read.v_roles r JOIN cloud_read.v_user_roles u ON u.role_id=r.id WHERE u.user_id=$1::uuid ORDER BY r.name,r.id;

-- name: ActorArenaRoles
-- params: actor_id uuid, arena_id uuid
SELECT to_jsonb(r) FROM cloud_read.v_roles r JOIN cloud_read.v_arena_user_roles u ON u.role_id=r.id WHERE u.user_id=$1::uuid AND u.arena_id=$2::uuid ORDER BY r.name,r.id;

-- name: ParticipantCreate
-- params: new_user_id uuid, name text, phone text nullable, email text nullable, password_hash text nullable
INSERT INTO iam.users(id,name,phone,email,password_hash) VALUES($1::uuid,$2,$3,lower(btrim($4)),$5) RETURNING jsonb_build_object('id',id,'version',version);

-- name: ParticipantAssignRole
-- params: user_id uuid
INSERT INTO iam.user_roles(user_id,role_id) SELECT $1::uuid,id FROM iam.roles WHERE name='PESERTA' ON CONFLICT(user_id,role_id) DO NOTHING RETURNING jsonb_build_object('id',id);

-- name: ParticipantList
-- params: cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE v.deleted_at IS NULL AND EXISTS(SELECT FROM cloud_read.v_user_roles ur JOIN cloud_read.v_roles r ON r.id=ur.role_id WHERE ur.user_id=v.id AND r.name='PESERTA') AND ($1::uuid IS NULL OR v.id>$1) ORDER BY v.id LIMIT $2::int;

-- name: MasterUserGet
-- params: user_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_users v WHERE id=$1::uuid AND deleted_at IS NULL;

-- name: MasterUserLock
-- params: user_id uuid
SELECT jsonb_build_object('id',id,'version',version,'deleted_at',deleted_at,'is_active',is_active,'is_eo',is_eo) FROM iam.users WHERE id=$1::uuid FOR UPDATE;

-- name: MasterUserUpdate
-- params: user_id uuid, expected_version int, patch jsonb (name/phone/email/is_active/is_eo)
UPDATE iam.users SET name=CASE WHEN $3::jsonb ? 'name' THEN ($3::jsonb->>'name')::text ELSE name END,
 phone=CASE WHEN $3::jsonb ? 'phone' THEN ($3::jsonb->>'phone')::text ELSE phone END,
 email=CASE WHEN $3::jsonb ? 'email' THEN lower(btrim(($3::jsonb->>'email')::text)) ELSE email END,
 is_active=CASE WHEN $3::jsonb ? 'is_active' THEN ($3::jsonb->>'is_active')::boolean ELSE is_active END,
 is_eo=CASE WHEN $3::jsonb ? 'is_eo' THEN ($3::jsonb->>'is_eo')::boolean ELSE is_eo END WHERE id=$1::uuid AND version=$2::int AND deleted_at IS NULL RETURNING jsonb_build_object('id',id,'version',version);

-- name: MasterUserSoftDelete
-- params: user_id uuid, expected_version int
UPDATE iam.users SET deleted_at=coalesce(deleted_at,CURRENT_TIMESTAMP) WHERE id=$1::uuid AND version=$2::int RETURNING jsonb_build_object('id',id,'version',version);

-- name: MasterUserHistory
-- params: user_id uuid, after_version int, limit int
SELECT to_jsonb(v)||jsonb_build_object('actor_name',u.name) FROM cloud_read.v_user_master_history v LEFT JOIN cloud_read.v_users u ON u.id=v.actor_id WHERE v.user_id=$1::uuid AND v.version>$2::int ORDER BY v.version,v.id LIMIT $3::int;

-- name: AssignArenaRole
-- params: user_id uuid, arena_id uuid, role_name text
INSERT INTO iam.arena_user_roles(user_id,arena_id,role_id) SELECT u.id,$2::uuid,r.id FROM iam.users u CROSS JOIN iam.roles r WHERE u.id=$1::uuid AND u.deleted_at IS NULL AND u.is_active AND r.name=$3 ON CONFLICT(arena_id,user_id,role_id) DO NOTHING RETURNING jsonb_build_object('id',id);

-- name: AssignGlobalRole
-- params: user_id uuid, role_name text (ADMIN/OPERATOR/JURI/PESERTA); ADMIN-authorized UseCase only
INSERT INTO iam.user_roles(user_id,role_id) SELECT u.id,r.id FROM iam.users u CROSS JOIN iam.roles r WHERE u.id=$1::uuid AND u.deleted_at IS NULL AND u.is_active AND r.name=$2 AND r.name IN ('ADMIN','OPERATOR','JURI','PESERTA') ON CONFLICT(user_id,role_id) DO NOTHING RETURNING jsonb_build_object('id',id);

-- name: RemoveArenaRole
-- params: user_id uuid, arena_id uuid, role_name text
DELETE FROM iam.arena_user_roles u USING iam.roles r WHERE u.role_id=r.id AND u.user_id=$1::uuid AND u.arena_id=$2::uuid AND r.name=$3 RETURNING jsonb_build_object('id',u.id);
