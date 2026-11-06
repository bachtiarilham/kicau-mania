-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_users AS
SELECT * FROM cloud_read.v_users
ORDER BY name, id LIMIT $1 OFFSET $2;

PREPARE get_users AS
SELECT * FROM cloud_read.v_users WHERE id = $1;

PREPARE list_roles AS
SELECT * FROM cloud_read.v_roles
ORDER BY name, id LIMIT $1 OFFSET $2;

PREPARE get_roles AS
SELECT * FROM cloud_read.v_roles WHERE id = $1;

PREPARE list_user_auth_providers AS
SELECT * FROM cloud_read.v_user_auth_providers WHERE user_id = $1
ORDER BY provider, id LIMIT $2 OFFSET $3;

PREPARE get_user_auth_providers AS
SELECT * FROM cloud_read.v_user_auth_providers WHERE id = $1;

PREPARE list_user_sessions AS
SELECT * FROM cloud_read.v_user_sessions WHERE user_id = $1
ORDER BY expires_at DESC, id LIMIT $2 OFFSET $3;

PREPARE get_user_sessions AS
SELECT * FROM cloud_read.v_user_sessions WHERE id = $1;

PREPARE revoke_user_session AS
UPDATE iam.user_sessions SET revoked_at = CURRENT_TIMESTAMP
WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL RETURNING id;
