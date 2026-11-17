-- name: DesktopEngineCreate
-- params: id uuid, arena_id uuid, token_hash text, actor uuid
INSERT INTO media.ingest_engines(id,arena_id,token_hash,created_by) VALUES($1::uuid,$2::uuid,$3,$4::uuid) ON CONFLICT(id) DO UPDATE SET token_hash=excluded.token_hash,revoked_at=NULL WHERE ingest_engines.arena_id=excluded.arena_id RETURNING jsonb_build_object('id',id,'arena_id',arena_id);

-- name: DesktopEngineGet
-- params: id uuid
SELECT to_jsonb(e)-'token_hash' FROM media.ingest_engines e WHERE id=$1::uuid AND revoked_at IS NULL;

-- name: DesktopEngineAuth
-- params: token_hash text
SELECT to_jsonb(e)-'token_hash' FROM media.ingest_engines e WHERE token_hash=$1 AND revoked_at IS NULL;

-- name: DesktopEngineRevoke
-- params: id uuid
UPDATE media.ingest_engines SET revoked_at=COALESCE(revoked_at,now()) WHERE id=$1::uuid;

-- name: DesktopSessionGet
-- params: engine uuid, session text
SELECT to_jsonb(s) FROM media.ingest_sessions s WHERE engine_id=$1::uuid AND local_session_id=$2;

-- name: DesktopSessionCreate
-- params: id uuid, engine uuid, session text, round uuid, actor uuid
INSERT INTO media.ingest_sessions(id,engine_id,local_session_id,round_id,created_by) VALUES($1::uuid,$2::uuid,$3,$4::uuid,$5::uuid) RETURNING to_jsonb(ingest_sessions);

-- name: DesktopMappingValidate
-- params: registration uuid, arena uuid, round uuid, gantangan uuid
SELECT jsonb_build_object('registration_id',t.id,'gantangan_id',t.gantangan_id,'user_id',t.user_id,'bird_id',t.bird_id,'registrant_name',u.name,'bird_name',b.name) FROM competition.registrations t JOIN iam.users u ON u.id=t.user_id JOIN asset.birds b ON b.id=t.bird_id WHERE t.id=$1::uuid AND t.arena_id=$2::uuid AND t.round_id=$3::uuid AND t.gantangan_id=$4::uuid AND t.status='CONFIRMED' AND t.payment_status='VERIFIED' FOR SHARE OF t;

-- name: DesktopMappingCreate
-- params: ingestion uuid, channel text, registration uuid, gantangan uuid, snapshot jsonb
INSERT INTO media.ingest_mappings VALUES($1::uuid,$2,$3::uuid,$4::uuid,$5::jsonb);

-- name: DesktopMappingList
-- params: ingestion uuid
SELECT to_jsonb(m) FROM media.ingest_mappings m WHERE ingestion_id=$1::uuid ORDER BY channel_id;

-- name: DesktopAssetCreate
-- params: id uuid, engine uuid, ingestion uuid, channel text, local_recording text, object_key text, size bigint, sha256 text, started timestamptz, ended timestamptz
INSERT INTO media.recording_assets(id,engine_id,ingestion_id,channel_id,local_recording_id,object_key,size_bytes,sha256,started_at,ended_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6,$7::bigint,$8,$9::timestamptz,$10::timestamptz) ON CONFLICT(engine_id,local_recording_id) DO UPDATE SET local_recording_id=excluded.local_recording_id RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetLock
-- params: id uuid, engine uuid
SELECT to_jsonb(a) FROM media.recording_assets a WHERE id=$1::uuid AND engine_id=$2::uuid FOR UPDATE;

-- name: DesktopAssetBegin
-- params: id uuid, multipart text
UPDATE media.recording_assets SET multipart_id=$2,state='UPLOADING',parts='[]',updated_at=now() WHERE id=$1::uuid RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetParts
-- params: id uuid, parts jsonb
UPDATE media.recording_assets SET parts=$2::jsonb,updated_at=now() WHERE id=$1::uuid RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetComplete
-- params: id uuid
UPDATE media.recording_assets SET state='UPLOADED',updated_at=now() WHERE id=$1::uuid RETURNING to_jsonb(recording_assets);

-- name: DesktopAssetRead
-- params: id uuid
SELECT to_jsonb(a)||jsonb_build_object('arena_id',e.arena_id,'registration_id',m.registration_id,'user_id',t.user_id,'round_id',s.round_id,'snapshot',m.snapshot) FROM media.recording_assets a JOIN media.ingest_engines e ON e.id=a.engine_id JOIN media.ingest_sessions s ON s.id=a.ingestion_id JOIN media.ingest_mappings m ON m.ingestion_id=a.ingestion_id AND m.channel_id=a.channel_id JOIN competition.registrations t ON t.id=m.registration_id WHERE a.id=$1::uuid;

-- name: DesktopAssetList
-- params: arena uuid, round uuid nullable, session text nullable, channel text nullable, cursor uuid nullable, limit int, user uuid nullable
SELECT to_jsonb(a)-'multipart_id'||jsonb_build_object('registration_id',m.registration_id,'round_id',s.round_id,'local_session_id',s.local_session_id,'snapshot',m.snapshot) FROM media.recording_assets a JOIN media.ingest_engines e ON e.id=a.engine_id JOIN media.ingest_sessions s ON s.id=a.ingestion_id JOIN media.ingest_mappings m ON m.ingestion_id=a.ingestion_id AND m.channel_id=a.channel_id JOIN competition.registrations t ON t.id=m.registration_id WHERE e.arena_id=$1::uuid AND ($2::uuid IS NULL OR s.round_id=$2) AND ($3::text IS NULL OR s.local_session_id=$3) AND ($4::text IS NULL OR a.channel_id=$4) AND ($5::uuid IS NULL OR a.id>$5) AND ($7::uuid IS NULL OR t.user_id=$7) ORDER BY a.id LIMIT $6::int;
