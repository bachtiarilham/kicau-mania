-- Rollback removes only the new ingestion metadata, never storage objects.
DROP TABLE media.recording_assets;
DROP TABLE media.ingest_mappings;
DROP TABLE media.ingest_sessions;
DROP TABLE media.ingest_engines;
