-- Examples for one session. Repository Go can use the SQL bodies directly.

PREPARE list_score_criteria AS
SELECT * FROM cloud_read.v_score_criteria
ORDER BY criteria_name, id LIMIT $1 OFFSET $2;

PREPARE get_score_criteria AS
SELECT * FROM cloud_read.v_score_criteria WHERE id = $1;

PREPARE list_round_scores AS
SELECT * FROM cloud_read.v_round_scores WHERE round_id = $1
ORDER BY round_participant_id, judge_user_id, criteria_id, id LIMIT $2 OFFSET $3;

PREPARE get_round_scores AS
SELECT * FROM cloud_read.v_round_scores WHERE id = $1;

PREPARE list_round_score_summary AS
SELECT * FROM cloud_read.mv_round_score_summary WHERE round_id = $1
ORDER BY round_participant_id LIMIT $2 OFFSET $3;

PREPARE get_round_score_summary AS
SELECT * FROM cloud_read.mv_round_score_summary WHERE round_participant_id = $1;
