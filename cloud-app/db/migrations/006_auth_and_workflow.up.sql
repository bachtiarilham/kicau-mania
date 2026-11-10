-- Apply in a transaction after 005. No runtime DDL.
DO $$ BEGIN
 IF EXISTS(SELECT FROM judging.round_scores) OR EXISTS(SELECT FROM judging.score_revisions) THEN
  RAISE EXCEPTION 'Existing scores need a separate assessment-history import before 006';
 END IF;
END $$;
ALTER TABLE iam.users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE iam.users ADD COLUMN email_verified_at timestamptz;
UPDATE iam.users SET email=lower(btrim(email)) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX users_email_normalized_uidx ON iam.users(lower(email)) WHERE email IS NOT NULL;
ALTER TABLE iam.user_sessions ADD COLUMN access_token_hash bytea,
 ADD COLUMN access_expires_at timestamptz, ADD COLUMN updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;
UPDATE iam.user_sessions SET revoked_at=CURRENT_TIMESTAMP WHERE revoked_at IS NULL;
CREATE UNIQUE INDEX user_sessions_access_hash_uidx ON iam.user_sessions(access_token_hash) WHERE access_token_hash IS NOT NULL;
CREATE UNIQUE INDEX user_sessions_refresh_hash_uidx ON iam.user_sessions(refresh_token_hash);
CREATE TABLE iam.account_tokens (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), user_id uuid NOT NULL REFERENCES iam.users(id) ON DELETE CASCADE,
 purpose text NOT NULL CHECK(purpose IN ('VERIFY_EMAIL','RESET_PASSWORD')),
 token_hash bytea NOT NULL UNIQUE, expires_at timestamptz NOT NULL, consumed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX account_tokens_pending_uidx ON iam.account_tokens(user_id,purpose) WHERE consumed_at IS NULL;
CREATE UNIQUE INDEX auth_provider_user_uidx ON iam.user_auth_providers(user_id,provider);
ALTER TABLE asset.owners ADD COLUMN created_by_user_id uuid REFERENCES iam.users(id) ON DELETE RESTRICT,
 ADD COLUMN created_in_arena_id uuid REFERENCES asset.arenas(id) ON DELETE RESTRICT;
ALTER TABLE asset.arenas ADD COLUMN criteria_locked_at timestamptz;
UPDATE asset.arenas a SET criteria_locked_at=CURRENT_TIMESTAMP WHERE EXISTS
 (SELECT FROM competition.rounds r WHERE r.arena_id=a.id AND r.phase<>'PREPARED');
COMMENT ON TABLE judging.score_criteria IS 'Shared arena criteria; UseCase serializes arena configuration and locks the list on first round start.';
CREATE TABLE judging.participant_assessments (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), arena_id uuid NOT NULL, round_id uuid NOT NULL,
 registration_item_id uuid NOT NULL, round_judge_id uuid NOT NULL,
 version integer NOT NULL CHECK(version>0), updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(registration_item_id,round_judge_id),
 FOREIGN KEY(registration_item_id,round_id,arena_id) REFERENCES competition.registration_items(id,round_id,arena_id),
 FOREIGN KEY(round_judge_id,round_id,arena_id) REFERENCES judging.round_judges(id,round_id,arena_id)
);
ALTER TABLE judging.round_scores ADD CONSTRAINT round_scores_assessment_fkey
 FOREIGN KEY(registration_item_id,round_judge_id)
 REFERENCES judging.participant_assessments(registration_item_id,round_judge_id);
CREATE TABLE judging.score_operations (
 actor_user_id uuid NOT NULL REFERENCES iam.users(id), operation_id uuid NOT NULL,
 payload_hash bytea NOT NULL, assessment_id uuid NOT NULL REFERENCES judging.participant_assessments(id),
 result jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(actor_user_id,operation_id)
);
CREATE TABLE iam.workflow_seed_roles(role_id uuid PRIMARY KEY REFERENCES iam.roles(id) ON DELETE CASCADE);
WITH inserted AS (
 INSERT INTO iam.roles(name) VALUES ('superadmin'),('admin_arena'),('operator'),('juri'),('peserta'),('penjoki')
 ON CONFLICT(name) DO NOTHING RETURNING id
) INSERT INTO iam.workflow_seed_roles(role_id) SELECT id FROM inserted;
REVOKE ALL ON iam.workflow_seed_roles FROM PUBLIC;
REVOKE ALL ON iam.account_tokens,judging.score_operations FROM PUBLIC;
