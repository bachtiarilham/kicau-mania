-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_round_recordings AS
SELECT * FROM cloud_read.v_round_recordings WHERE round_id = $1
ORDER BY created_at DESC NULLS LAST, id LIMIT $2 OFFSET $3;

PREPARE get_round_recordings AS
SELECT * FROM cloud_read.v_round_recordings WHERE id = $1;
