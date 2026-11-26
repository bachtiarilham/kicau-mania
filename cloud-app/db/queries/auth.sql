-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: LoginAccountByEmail
-- params: email text
SELECT jsonb_build_object('id',id,'email',email,'password_hash',password_hash,'email_verified_at',email_verified_at,'is_active',is_active) FROM iam.users WHERE lower(btrim(email))=lower(btrim($1)) AND deleted_at IS NULL;

-- name: LoginUserLock
-- params: user_id bigint
SELECT jsonb_build_object('id',id,'is_active',is_active,'email_verified_at',email_verified_at) FROM iam.users WHERE id=$1::text::bigint AND deleted_at IS NULL FOR UPDATE;

-- name: LoginGoogleAccount
-- params: verified_google_subject text
SELECT jsonb_build_object('id',u.id,'is_active',u.is_active,'email_verified_at',u.email_verified_at) FROM iam.user_auth_providers p JOIN iam.users u ON u.id=p.user_id WHERE p.provider='google' AND p.provider_user_id=$1 AND u.deleted_at IS NULL;

-- name: LoginGoogleLink
-- params: user_id bigint, verified_google_subject text
INSERT INTO iam.user_auth_providers(user_id,provider,provider_user_id) VALUES($1::text::bigint,'google',$2) RETURNING jsonb_build_object('id',id);

-- name: LoginExpireSessions
-- params: user_id bigint
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1::text::bigint AND revoked_at IS NULL AND expires_at<=CURRENT_TIMESTAMP;

-- name: LoginOtherDeviceCount
-- params: user_id bigint, device_id text
SELECT jsonb_build_object('count',count(*)) FROM iam.user_sessions WHERE user_id=$1::text::bigint AND device_id<>$2 AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP;

-- name: LoginSessionSave
-- params: user_id bigint, device_id text, expires_at timestamptz, user_agent text nullable
INSERT INTO iam.user_sessions(user_id,device_id,expires_at,user_agent) VALUES($1::text::bigint,$2,$3::timestamptz,$4) ON CONFLICT(user_id,device_id) WHERE revoked_at IS NULL DO UPDATE SET expires_at=excluded.expires_at,user_agent=excluded.user_agent RETURNING jsonb_build_object('id',id,'expires_at',expires_at);

-- name: LoginSessionValidate
-- params: verified_session_id bigint, verified_user_id bigint
SELECT jsonb_build_object('session_id',s.id,'user_id',u.id,'expires_at',s.expires_at) FROM iam.user_sessions s JOIN iam.users u ON u.id=s.user_id WHERE s.id=$1::text::bigint AND u.id=$2::text::bigint AND s.revoked_at IS NULL AND s.expires_at>CURRENT_TIMESTAMP AND u.deleted_at IS NULL AND u.is_active;

-- name: LoginSessionList
-- params: user_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_user_sessions v WHERE user_id=$1::text::bigint ORDER BY expires_at DESC,id;

-- name: LoginSessionRevoke
-- params: user_id bigint, session_id bigint nullable
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1::text::bigint AND ($2::text::bigint IS NULL OR id=$2) AND revoked_at IS NULL;

-- name: AccountVerifyEmail
-- params: user_id bigint
UPDATE iam.users SET email_verified_at=coalesce(email_verified_at,CURRENT_TIMESTAMP) WHERE id=$1::text::bigint AND deleted_at IS NULL RETURNING jsonb_build_object('id',id);

-- name: AccountSetPassword
-- params: user_id bigint, password_hash text
UPDATE iam.users SET password_hash=$2 WHERE id=$1::text::bigint AND deleted_at IS NULL RETURNING jsonb_build_object('id',id);
