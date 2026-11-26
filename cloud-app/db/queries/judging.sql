-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: ScoreCriterionList
-- params: arena_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::text::bigint AND ($2::text::bigint IS NULL OR id>$2) ORDER BY id LIMIT $3::int;

-- name: ScoreCriterionGet
-- params: arena_id bigint, id bigint
SELECT to_jsonb(v) FROM cloud_read.v_score_criteria v WHERE arena_id=$1::text::bigint AND id=$2::text::bigint;

-- name: ScoreCriterionCreate
-- params: arena_id bigint, criteria_name text
INSERT INTO judging.score_criteria(arena_id,criteria_name) VALUES($1::text::bigint,$2::text) RETURNING jsonb_build_object('id',id);

-- name: ScoreCriterionUpdate
-- params: arena_id bigint, id bigint, patch jsonb
UPDATE judging.score_criteria SET criteria_name=CASE WHEN $3::jsonb ? 'criteria_name' THEN ($3::jsonb->>'criteria_name')::text ELSE criteria_name END WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: ScoreCriterionDelete
-- params: arena_id bigint, id bigint
DELETE FROM judging.score_criteria WHERE arena_id=$1::text::bigint AND id=$2::text::bigint RETURNING jsonb_build_object('id',id);

-- name: RoundJudgeAssign
-- params: arena_id bigint, round_id bigint, judge_user_id bigint
INSERT INTO judging.round_judges(round_id,judge_user_id) SELECT r.id,u.id FROM competition.rounds r CROSS JOIN iam.users u WHERE r.id=$2::text::bigint AND r.arena_id=$1::text::bigint AND u.id=$3::text::bigint AND u.deleted_at IS NULL AND u.is_active ON CONFLICT(round_id,judge_user_id) DO NOTHING RETURNING jsonb_build_object('id',id);

-- name: RoundJudgeRemove
-- params: arena_id bigint, round_id bigint, judge_user_id bigint
DELETE FROM judging.round_judges j USING competition.rounds r WHERE j.round_id=r.id AND r.arena_id=$1::text::bigint AND r.id=$2::text::bigint AND j.judge_user_id=$3::text::bigint RETURNING jsonb_build_object('id',j.id);

-- name: RoundJudgeList
-- params: arena_id bigint, round_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_round_judges v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint ORDER BY judge_user_id,id;

-- name: JudgeRoundList
-- params: judge_user_id bigint, cursor bigint nullable, limit int
SELECT to_jsonb(v) FROM cloud_read.v_rounds v WHERE EXISTS(SELECT FROM cloud_read.v_round_judges j WHERE j.round_id=v.id AND j.judge_user_id=$1::text::bigint) AND EXISTS(SELECT FROM iam.arena_user_roles a JOIN iam.roles r ON r.id=a.role_id WHERE a.user_id=$1::text::bigint AND a.arena_id=v.arena_id AND r.name='JURI') AND ($2::text::bigint IS NULL OR v.id>$2) ORDER BY v.id LIMIT $3::int;

-- name: ScoreList
-- params: arena_id bigint, registration_id bigint, judge_user_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_round_scores v WHERE arena_id=$1::text::bigint AND registration_id=$2::text::bigint AND user_id=$3::text::bigint ORDER BY criteria_id,id;

-- name: ScoreUpsert
-- params: arena_id bigint, round_id bigint, registration_id bigint, judge_user_id bigint, criterion_id bigint, score numeric
INSERT INTO judging.round_scores(arena_id,round_id,registration_id,user_id,criteria_id,score_value) SELECT r.arena_id,r.id,t.id,u.id,$5::text::bigint,$6::numeric FROM competition.rounds r JOIN competition.registrations t ON t.round_id=r.id CROSS JOIN iam.users u WHERE r.arena_id=$1::text::bigint AND r.id=$2::text::bigint AND t.id=$3::text::bigint AND u.id=$4::text::bigint AND u.deleted_at IS NULL AND u.is_active AND r.status='JUDGING' AND t.status='CONFIRMED' AND t.payment_status='VERIFIED' AND t.participant_status='ACTIVE' ON CONFLICT(round_id,registration_id,user_id,criteria_id) DO UPDATE SET score_value=excluded.score_value,updated_at=clock_timestamp() RETURNING jsonb_build_object('id',id,'score_value',score_value);

-- name: RoundLiveSummary
-- params: arena_id bigint, round_id bigint
SELECT to_jsonb(v) FROM cloud_read.v_round_score_summary v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint ORDER BY average_score DESC NULLS LAST,registration_id;

-- name: RoundCompleteness
-- params: arena_id bigint, round_id bigint
SELECT jsonb_build_object('participant_count',count(*),'complete_count',count(*) FILTER(WHERE is_complete),'all_complete',count(*)>0 AND bool_and(is_complete)) FROM cloud_read.v_round_score_summary WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint;
