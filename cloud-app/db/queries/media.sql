-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: MediaRecordingList
-- params: arena_id bigint, round_id bigint nullable, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_round_recordings v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR round_id=$2) AND ($3::text::bigint IS NULL OR id>$3) ORDER BY id LIMIT $4::int;

-- name: MediaRecordingGet
-- params: arena_id bigint, recording_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_round_recordings v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: MediaRecordingSave
-- params: registration_id bigint, video_url text nullable, audio_url text nullable, recording_status text
INSERT INTO media.round_recordings(registration_id,video_url,audio_url,recording_status) VALUES($1::text::bigint,$2,$3,$4) ON CONFLICT(registration_id) DO UPDATE SET video_url=excluded.video_url,audio_url=excluded.audio_url,recording_status=excluded.recording_status,updated_at=CURRENT_TIMESTAMP RETURNING jsonb_build_object('id',id);

-- name: MediaRecordingDelivery
-- params: recording_id bigint, delivery_status text
UPDATE media.round_recordings SET delivery_status=$2::text,sent_at=CASE WHEN $2::text='SENT' THEN CURRENT_TIMESTAMP END,updated_at=CURRENT_TIMESTAMP WHERE id=$1::text::bigint RETURNING jsonb_build_object('id',id);

-- name: MediaRecordingDelete
-- params: arena_id bigint, recording_id bigint
DELETE FROM media.round_recordings m USING competition.registrations t WHERE m.registration_id=t.id AND t.arena_id=$1::text::bigint AND m.id=$2::text::bigint RETURNING jsonb_build_object('id',m.id);

-- name: MediaRecordingSourceInternal
-- params: recording_id bigint
SELECT jsonb_build_object('id',m.id,'registration_id',m.registration_id,'video_url',m.video_url,'audio_url',m.audio_url,'recipient_phone',u.phone) FROM media.round_recordings m JOIN competition.registrations t ON t.id=m.registration_id JOIN iam.users u ON u.id=t.user_id WHERE m.id=$1::text::bigint;
