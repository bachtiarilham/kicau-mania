-- name: DesktopEngineCreate
-- params: id bigint, arena_id bigint, token_hash text, actor bigint
INSERT INTO media.ingest_engines(pairing_key,arena_id,token_hash,created_by) VALUES($1::uuid,$2::text::bigint,$3,$4::text::bigint) ON CONFLICT(pairing_key) DO UPDATE SET token_hash=excluded.token_hash,revoked_at=NULL WHERE ingest_engines.arena_id=excluded.arena_id RETURNING jsonb_build_object('id',id,'arena_id',arena_id);

-- name: DesktopEngineRotate
UPDATE media.ingest_engines SET token_hash=$3,revoked_at=NULL WHERE id=$1::text::bigint AND arena_id=$2::text::bigint RETURNING jsonb_build_object('id',id,'arena_id',arena_id);

-- name: DesktopAssetNextID
SELECT jsonb_build_object('id',nextval(pg_get_serial_sequence('media.recording_assets','id')));

-- name: DesktopPublicWatch
SELECT jsonb_build_object('id',a.id,'state',a.state,'object_key',a.object_key,'channel_id',a.channel_id,'snapshot',m.snapshot) FROM media.recording_assets a JOIN media.ingest_mappings m USING(ingestion_id,channel_id) WHERE a.public_code=$1::uuid;


-- name: DesktopEngineGet
-- params: id bigint
SELECT to_jsonb(e)-'token_hash' FROM media.ingest_engines e WHERE id=$1::text::bigint AND revoked_at IS NULL;

-- name: DesktopEngineAuth
-- params: token_hash text
SELECT to_jsonb(e)-'token_hash' FROM media.ingest_engines e WHERE token_hash=$1 AND revoked_at IS NULL;

-- name: DesktopEngineRevoke
-- params: id bigint
UPDATE media.ingest_engines SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1::text::bigint;

-- name: DesktopSessionGet
-- params: engine bigint, session text
SELECT to_jsonb(s) FROM media.ingest_sessions s WHERE engine_id=$1::text::bigint AND local_session_id=$2;

-- name: DesktopSessionCreate
-- params: id bigint, engine bigint, session text, round bigint, actor bigint
INSERT INTO media.ingest_sessions(engine_id,local_session_id,round_id,created_by) VALUES($1::text::bigint,$2,$3::text::bigint,$4::text::bigint) RETURNING to_jsonb(ingest_sessions);

-- name: DesktopMappingValidate
-- params: registration bigint, arena bigint, round bigint, gantangan bigint
SELECT jsonb_build_object('registration_id',t.id,'gantangan_id',t.gantangan_id,'user_id',t.user_id,'bird_id',t.bird_id,'registrant_name',u.name,'bird_name',b.name) FROM competition.registrations t JOIN iam.users u ON u.id=t.user_id JOIN asset.birds b ON b.id=t.bird_id WHERE t.id=$1::text::bigint AND t.arena_id=$2::text::bigint AND t.round_id=$3::text::bigint AND t.gantangan_id=$4::text::bigint AND t.status='CONFIRMED' AND t.payment_status='VERIFIED' FOR SHARE OF t;

-- name: DesktopMappingCreate
-- params: ingestion bigint, channel text, registration bigint, gantangan bigint, snapshot jsonb
INSERT INTO media.ingest_mappings VALUES($1::text::bigint,$2,$3::text::bigint,$4::text::bigint,$5::jsonb);

-- name: DesktopMappingList
-- params: ingestion bigint
SELECT to_jsonb(m) FROM media.ingest_mappings m WHERE ingestion_id=$1::text::bigint ORDER BY channel_id;

-- name: DesktopAssetCreate
-- params: id bigint, engine bigint, ingestion bigint, channel text, local_recording text, object_key text, size bigint, sha256 text, started timestamptz, ended timestamptz
INSERT INTO media.recording_assets(id,engine_id,ingestion_id,channel_id,local_recording_id,object_key,size_bytes,sha256,started_at,ended_at) VALUES($1::text::bigint,$2::text::bigint,$3::text::bigint,$4,$5,$6,$7::bigint,$8,$9::timestamptz,$10::timestamptz) ON CONFLICT(engine_id,local_recording_id) DO UPDATE SET local_recording_id=excluded.local_recording_id RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetLock
-- params: id bigint, engine bigint
SELECT to_jsonb(a) FROM media.recording_assets a WHERE id=$1::text::bigint AND engine_id=$2::text::bigint FOR UPDATE;

-- name: DesktopAssetBegin
-- params: id bigint, multipart text
UPDATE media.recording_assets SET multipart_id=$2,state='UPLOADING',parts='[]',updated_at=now() WHERE id=$1::text::bigint RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetParts
-- params: id bigint, parts jsonb
UPDATE media.recording_assets SET parts=$2::jsonb,updated_at=now() WHERE id=$1::text::bigint RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetComplete
-- params: id bigint
UPDATE media.recording_assets SET state='UPLOADED',updated_at=now() WHERE id=$1::text::bigint RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetRead
-- params: id bigint
SELECT to_jsonb(a)||jsonb_build_object('arena_id',e.arena_id,'registration_id',m.registration_id,'user_id',t.user_id,'round_id',s.round_id,'snapshot',m.snapshot,'user_phone',COALESCE(t.owner_phone, t.participant_phone, u.phone, '')) FROM media.recording_assets a JOIN media.ingest_engines e ON e.id=a.engine_id JOIN media.ingest_sessions s ON s.id=a.ingestion_id JOIN media.ingest_mappings m ON m.ingestion_id=a.ingestion_id AND m.channel_id=a.channel_id JOIN competition.registrations t ON t.id=m.registration_id LEFT JOIN iam.users u ON u.id=t.user_id WHERE a.id=$1::text::bigint;

-- name: DesktopAssetList
-- params: arena bigint, round bigint nullable, session text nullable, channel text nullable, cursor bigint nullable, limit int, user bigint nullable
SELECT to_jsonb(a)-'multipart_id'||jsonb_build_object('registration_id',m.registration_id,'round_id',s.round_id,'local_session_id',s.local_session_id,'snapshot',m.snapshot) FROM media.recording_assets a JOIN media.ingest_engines e ON e.id=a.engine_id JOIN media.ingest_sessions s ON s.id=a.ingestion_id JOIN media.ingest_mappings m ON m.ingestion_id=a.ingestion_id AND m.channel_id=a.channel_id JOIN competition.registrations t ON t.id=m.registration_id WHERE e.arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR s.round_id=$2) AND ($3::text IS NULL OR s.local_session_id=$3) AND ($4::text IS NULL OR a.channel_id=$4) AND ($5::text::bigint IS NULL OR a.id>$5) AND ($7::text::bigint IS NULL OR t.user_id=$7) ORDER BY a.id LIMIT $6::int;
