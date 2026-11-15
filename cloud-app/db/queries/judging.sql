-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: ScoreCriterionList
-- params: arena_id uuid, cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::uuid AND ($2::uuid IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: ScoreCriterionGet
-- params: arena_id uuid, id uuid
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::uuid AND id=$2::uuid;

-- name: ScoreCriterionCreate
-- params: arena_id uuid, criteria_name text
INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES($1::uuid,$2::text) RETURNING jsonb_build_object('id',id);

-- name: ScoreCriterionUpdate
-- params: arena_id uuid, id uuid, patch jsonb
UPDATE judging.score_criteria SET criteria_name=CASE WHEN $3::jsonb ? 'criteria_name' THEN ($3::jsonb->>'criteria_name')::text ELSE criteria_name END WHERE arena_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: ScoreCriterionDelete
-- params: arena_id uuid, id uuid
DELETE FROM judging.score_criteria WHERE arena_id=$1::uuid AND id=$2::uuid RETURNING jsonb_build_object('id',id);

-- name: RoundJudgeAssign
-- params: arena_id uuid, round_id uuid, judge_user_id uuid
INSERT INTO judging.round_judges(round_id,judge_user_id) SELECT r.id,u.id FROM competition.rounds r CROSS JOIN iam.users u WHERE r.id=$2::uuid AND r.arena_id=$1::uuid AND u.id=$3::uuid AND u.deleted_at IS NULL AND u.is_active ON CONFLICT(round_id,judge_user_id) DO NOTHING RETURNING jsonb_build_object('id',id);

-- name: RoundJudgeRemove
-- params: arena_id uuid, round_id uuid, judge_user_id uuid
DELETE FROM judging.round_judges j USING competition.rounds r WHERE j.round_id=r.id AND r.arena_id=$1::uuid AND r.id=$2::uuid AND j.judge_user_id=$3::uuid RETURNING jsonb_build_object('id',j.id);

-- name: RoundJudgeList
-- params: arena_id uuid, round_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_round_judges v WHERE arena_id=$1::uuid AND round_id=$2::uuid ORDER BY judge_user_id,id;

-- name: JudgeRoundList
-- params: judge_user_id uuid, cursor uuid nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE EXISTS(SELECT FROM cloud_read.v_round_judges j WHERE j.round_id=v.id AND j.judge_user_id=$1::uuid) AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::uuid AND a.arena_id=v.arena_id AND r.name='JURI') AND ($2::uuid IS NULL OR v.id>$2) ORDER BY v.id LIMIT $3::int;

-- name: ScoreList
-- params: arena_id uuid, registration_id uuid, judge_user_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_round_scores v WHERE arena_id=$1::uuid AND registration_id=$2::uuid AND user_id=$3::uuid ORDER BY criteria_id,id;

-- name: ScoreUpsert
-- params: arena_id uuid, round_id uuid, registration_id uuid, judge_user_id uuid, criterion_id uuid, score numeric
INSERT INTO judging.round_scores(arena_id,round_id,registration_id,user_id,criteria_id,score_value) SELECT r.arena_id,r.id,t.id,u.id,$5::uuid,$6::numeric FROM competition.rounds r JOIN competition.registrations t ON t.round_id=r.id CROSS JOIN iam.users u WHERE r.arena_id=$1::uuid AND r.id=$2::uuid AND t.id=$3::uuid AND u.id=$4::uuid AND u.deleted_at IS NULL AND u.is_active AND r.status='JUDGING' AND t.status='CONFIRMED' AND t.payment_status='VERIFIED' AND t.participant_status='ACTIVE' ON CONFLICT(round_id,registration_id,user_id,criteria_id) DO UPDATE SET score_value=excluded.score_value,updated_at=clock_timestamp() RETURNING jsonb_build_object('id',id,'score_value',score_value);

-- name: RoundLiveSummary
-- params: arena_id uuid, round_id uuid
SELECT to_jsonb(v) FROM cloud_read.v_round_score_summary v WHERE arena_id=$1::uuid AND round_id=$2::uuid ORDER BY average_score DESC NULLS LAST,registration_id;

-- name: RoundCompleteness
-- params: arena_id uuid, round_id uuid
SELECT jsonb_build_object('participant_count',count(*),'complete_count',count(*) FILTER(WHERE is_complete),'all_complete',count(*)>0 AND bool_and(is_complete)) FROM cloud_read.v_round_score_summary WHERE arena_id=$1::uuid AND round_id=$2::uuid;
