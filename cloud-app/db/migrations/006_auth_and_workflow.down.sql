-- Refuse destructive downgrade once the new workflows have data. Roll back 007 first.
DO $$ BEGIN
 IF EXISTS(SELECT FROM iam.account_tokens) OR EXISTS(SELECT FROM judging.participant_assessments)
 OR EXISTS(SELECT FROM judging.score_operations) OR EXISTS(SELECT FROM iam.users WHERE email_verified_at IS NOT NULL OR password_hash IS NULL)
 OR EXISTS(SELECT FROM iam.user_sessions WHERE access_token_hash IS NOT NULL)
 OR EXISTS(SELECT FROM asset.owners WHERE created_by_user_id IS NOT NULL OR created_in_arena_id IS NOT NULL)
 OR EXISTS(SELECT FROM asset.arenas WHERE criteria_locked_at IS NOT NULL)
 OR EXISTS(SELECT FROM iam.user_roles WHERE role_id IN(SELECT role_id FROM iam.workflow_seed_roles))
 OR EXISTS(SELECT FROM iam.arena_user_roles WHERE role_id IN(SELECT role_id FROM iam.workflow_seed_roles))
 THEN RAISE EXCEPTION 'Workflow rollback requires unused supporting structures; restore backup for populated database'; END IF;
END $$;
DROP TABLE judging.score_operations;
ALTER TABLE judging.round_scores DROP CONSTRAINT round_scores_assessment_fkey;
DROP TABLE judging.participant_assessments;
DROP INDEX iam.auth_provider_user_uidx;
DROP TABLE iam.account_tokens;
DROP INDEX iam.user_sessions_access_hash_uidx;
DROP INDEX iam.user_sessions_refresh_hash_uidx;
ALTER TABLE iam.user_sessions DROP COLUMN access_token_hash,DROP COLUMN access_expires_at,DROP COLUMN updated_at;
DROP INDEX iam.users_email_normalized_uidx;
ALTER TABLE iam.users DROP COLUMN email_verified_at, ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE asset.owners DROP COLUMN created_by_user_id,DROP COLUMN created_in_arena_id;
ALTER TABLE asset.arenas DROP COLUMN criteria_locked_at;
-- Remove only seed rows owned by this migration. Pre-existing canonical roles are retained.
DELETE FROM iam.roles WHERE id IN(SELECT role_id FROM iam.workflow_seed_roles);
DROP TABLE iam.workflow_seed_roles;
