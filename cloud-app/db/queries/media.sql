-- name: RegistrationRecordings
SELECT to_jsonb(m) FROM cloud_read.v_round_recordings m JOIN cloud_read.v_registration_items i ON i.id=m.registration_item_id WHERE i.registration_id=$1::uuid ORDER BY m.id;
