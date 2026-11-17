-- Additive desktop ingestion. Historical round_recordings is retained unchanged.
CREATE TABLE media.ingest_engines (
 id uuid PRIMARY KEY, arena_id uuid NOT NULL REFERENCES asset.arenas(id),
 token_hash text NOT NULL UNIQUE, created_by uuid NOT NULL REFERENCES iam.users(id),
 revoked_at timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE media.ingest_sessions (
 id uuid PRIMARY KEY, engine_id uuid NOT NULL REFERENCES media.ingest_engines(id),
 local_session_id text NOT NULL, round_id uuid NOT NULL REFERENCES competition.rounds(id),
 created_by uuid NOT NULL REFERENCES iam.users(id), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(engine_id,local_session_id)
);
CREATE TABLE media.ingest_mappings (
 ingestion_id uuid NOT NULL REFERENCES media.ingest_sessions(id), channel_id text NOT NULL,
 registration_id uuid NOT NULL REFERENCES competition.registrations(id),
 gantangan_id uuid NOT NULL REFERENCES asset.gantangan(id), snapshot jsonb NOT NULL,
 PRIMARY KEY(ingestion_id,channel_id), UNIQUE(ingestion_id,registration_id)
);
CREATE TABLE media.recording_assets (
 id uuid PRIMARY KEY, engine_id uuid NOT NULL REFERENCES media.ingest_engines(id),
 ingestion_id uuid NOT NULL, channel_id text NOT NULL, local_recording_id text NOT NULL,
 object_key text NOT NULL UNIQUE, size_bytes bigint NOT NULL CHECK(size_bytes>0 AND size_bytes<=1099511627776),
 sha256 text NOT NULL CHECK(sha256 ~ '^[0-9a-f]{64}$'),
 started_at timestamptz NOT NULL, ended_at timestamptz NOT NULL CHECK(ended_at>=started_at),
 state text NOT NULL DEFAULT 'QUEUED' CHECK(state IN ('QUEUED','UPLOADING','UPLOADED')),
 multipart_id text, parts jsonb NOT NULL DEFAULT '[]',
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(engine_id,local_recording_id),
 FOREIGN KEY(ingestion_id,channel_id) REFERENCES media.ingest_mappings(ingestion_id,channel_id)
);
CREATE INDEX recording_assets_ingestion ON media.recording_assets(ingestion_id,id);
GRANT SELECT,INSERT,UPDATE,DELETE ON media.ingest_engines,media.ingest_sessions,media.ingest_mappings,media.recording_assets TO kicau_cloud_app;
