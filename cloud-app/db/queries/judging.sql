-- Named statements; placeholders are bound by pgx. No PREPARE execution required.

-- name: EntityScoreCriteriaGet
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE id=$1::uuid;

-- name: EntityScoreCriteriaList
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE id::text>$1 AND ($2::text='' OR arena_id::text=$2) ORDER BY id LIMIT $3;

-- name: EntityScoreCriteriaCreate
INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES(($1::jsonb->>'arena_id')::uuid,($1::jsonb->>'criteria_name')::text) RETURNING jsonb_build_object('id',id);

-- name: EntityScoreCriteriaUpdate
UPDATE judging.score_criteria SET arena_id=CASE WHEN $2::jsonb?'arena_id' THEN ($2->>'arena_id')::uuid ELSE arena_id END,criteria_name=CASE WHEN $2::jsonb?'criteria_name' THEN ($2->>'criteria_name')::text ELSE criteria_name END WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: EntityScoreCriteriaDelete
DELETE FROM judging.score_criteria WHERE id=$1::uuid RETURNING jsonb_build_object('id',id);

-- name: JudgeAssignment
SELECT to_jsonb(v) FROM cloud_read.v_round_judges v WHERE round_id=$1::uuid AND judge_user_id=$2::uuid;

-- name: JudgeRounds
SELECT to_jsonb(r) FROM cloud_read.v_rounds r JOIN cloud_read.v_round_judges j ON j.round_id=r.id WHERE j.judge_user_id=$1::uuid AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles role ON role.id=a.role_id WHERE a.user_id=$1::uuid AND a.arena_id=r.arena_id AND role.name='juri') AND r.id::text>$2 ORDER BY r.id LIMIT $3;

-- name: JudgeItem
SELECT to_jsonb(v) FROM cloud_read.v_registration_items v WHERE id=$1::uuid;

-- name: AssessmentGet
SELECT to_jsonb(v) FROM cloud_read.v_participant_assessments v WHERE registration_item_id=$1::uuid AND round_judge_id=$2::uuid;

-- name: AssessmentSave
INSERT INTO judging.participant_assessments(arena_id,round_id,registration_item_id,round_judge_id,version) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5) ON CONFLICT(registration_item_id,round_judge_id) DO UPDATE SET version=excluded.version,updated_at=CURRENT_TIMESTAMP RETURNING jsonb_build_object('id',id,'version',version);

-- name: AssessmentScores
SELECT to_jsonb(s) FROM cloud_read.v_round_scores s WHERE registration_item_id=$1::uuid AND round_judge_id=$2::uuid ORDER BY criteria_id;

-- name: AssessmentHistory
SELECT to_jsonb(h)||jsonb_build_object('criteria_id',s.criteria_id) FROM cloud_read.v_score_revisions h JOIN cloud_read.v_round_scores s ON s.id=h.score_id WHERE s.registration_item_id=$1::uuid AND s.round_judge_id=$2::uuid ORDER BY h.revision_number,s.criteria_id;

-- name: ScoreUpsert
INSERT INTO judging.round_scores(arena_id,round_id,registration_item_id,round_judge_id,criteria_id,score_value,revision_number) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7) ON CONFLICT(registration_item_id,round_judge_id,criteria_id) DO UPDATE SET score_value=excluded.score_value,revision_number=excluded.revision_number RETURNING jsonb_build_object('id',id);

-- name: ScoreRevision
INSERT INTO judging.score_revisions(score_id,operation_id,revision_number,score_value) VALUES($1::uuid,$2::uuid,$3,$4);

-- name: ScoreOperation
SELECT to_jsonb(v)||jsonb_build_object('payload_hash',encode(payload_hash,'hex')) FROM judging.score_operations v WHERE actor_user_id=$1::uuid AND operation_id=$2::uuid;

-- name: ScoreOperationSave
INSERT INTO judging.score_operations(actor_user_id,operation_id,payload_hash,assessment_id,result) VALUES($1::uuid,$2::uuid,$3,$4::uuid,$5::jsonb);
