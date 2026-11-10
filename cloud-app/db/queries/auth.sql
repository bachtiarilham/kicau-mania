
-- name: AuthByEmail
SELECT to_jsonb(u) FROM iam.users u WHERE lower(email)=lower($1);

-- name: AuthByID
SELECT to_jsonb(u) FROM iam.users u WHERE id=$1::uuid;

-- name: LockUser
SELECT to_jsonb(u) FROM iam.users u WHERE id=$1::uuid FOR UPDATE;

-- name: CreateUser
INSERT INTO iam.users(name,email,phone,password_hash,email_verified_at,is_verified) VALUES($1,lower($2),$3,$4,CASE WHEN $5::boolean THEN CURRENT_TIMESTAMP END,$5) RETURNING jsonb_build_object('id',id);

-- name: VerifyEmail
UPDATE iam.users SET email_verified_at=CURRENT_TIMESTAMP,is_verified=true,updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid;

-- name: PasswordUpdate
UPDATE iam.users SET password_hash=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid;

-- name: TokenRetire
UPDATE iam.account_tokens SET consumed_at=CURRENT_TIMESTAMP WHERE user_id=$1::uuid AND purpose=$2 AND consumed_at IS NULL;

-- name: TokenCreate
INSERT INTO iam.account_tokens(user_id,purpose,token_hash,expires_at) VALUES($1::uuid,$2,$3,CURRENT_TIMESTAMP+$4::interval);

-- name: TokenFind
SELECT to_jsonb(t) FROM iam.account_tokens t WHERE token_hash=$1 AND purpose=$2 AND consumed_at IS NULL AND expires_at>CURRENT_TIMESTAMP;

-- name: TokenConsume
UPDATE iam.account_tokens SET consumed_at=CURRENT_TIMESTAMP WHERE id=$1::uuid AND consumed_at IS NULL AND expires_at>CURRENT_TIMESTAMP RETURNING jsonb_build_object('id',id);

-- name: SessionExpire
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1::uuid AND revoked_at IS NULL AND expires_at<=CURRENT_TIMESTAMP;

-- name: SessionCount
SELECT jsonb_build_object('count',count(*)) FROM iam.user_sessions WHERE user_id=$1::uuid AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP AND device_id<>$2::uuid;

-- name: SessionSave
INSERT INTO iam.user_sessions(user_id,device_id,refresh_token_hash,access_token_hash,access_expires_at,expires_at,user_agent,ip_address) VALUES($1::uuid,$2::uuid,$3,$4,CURRENT_TIMESTAMP+interval '15 minutes',CURRENT_TIMESTAMP+interval '30 days',$5,$6) ON CONFLICT(user_id,device_id) WHERE revoked_at IS NULL DO UPDATE SET refresh_token_hash=excluded.refresh_token_hash,access_token_hash=excluded.access_token_hash,access_expires_at=excluded.access_expires_at,expires_at=excluded.expires_at,user_agent=excluded.user_agent,ip_address=excluded.ip_address,updated_at=CURRENT_TIMESTAMP RETURNING jsonb_build_object('id',id);

-- name: SessionAuthenticate
SELECT jsonb_build_object('user_id',u.id,'session_id',s.id) FROM iam.user_sessions s JOIN iam.users u ON u.id=s.user_id WHERE s.access_token_hash=$1 AND s.access_expires_at>CURRENT_TIMESTAMP AND s.expires_at>CURRENT_TIMESTAMP AND s.revoked_at IS NULL AND u.is_active AND u.email_verified_at IS NOT NULL;

-- name: RefreshFind
SELECT to_jsonb(s) FROM iam.user_sessions s WHERE refresh_token_hash=$1 AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP;

-- name: RefreshRotate
UPDATE iam.user_sessions SET refresh_token_hash=$2,access_token_hash=$3,access_expires_at=CURRENT_TIMESTAMP+interval '15 minutes',updated_at=CURRENT_TIMESTAMP WHERE id=$1::uuid AND refresh_token_hash=$4 AND revoked_at IS NULL AND expires_at>CURRENT_TIMESTAMP RETURNING jsonb_build_object('id',id);

-- name: SessionList
SELECT to_jsonb(v) FROM cloud_read.v_user_sessions v WHERE user_id=$1::uuid ORDER BY created_at DESC,id;

-- name: SessionRevoke
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1::uuid AND ($2::text='' OR id::text=$2) AND revoked_at IS NULL;

-- name: SessionLogout
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE user_id=$1::uuid AND access_token_hash=$2 AND revoked_at IS NULL;

-- name: ProviderFind
SELECT jsonb_build_object('user_id',user_id) FROM iam.user_auth_providers WHERE provider='google' AND provider_user_id=$1;

-- name: ProviderLink
INSERT INTO iam.user_auth_providers(user_id,provider,provider_user_id) VALUES($1::uuid,'google',$2);
