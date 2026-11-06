CREATE MATERIALIZED VIEW cloud_read.mv_round_score_summary AS
WITH classified_scores AS (
    SELECT round_participant_id, judge_user_id, score_value,
           score_value BETWEEN 1 AND 10 AND score_value = trunc(score_value) AS is_valid
    FROM judging.round_scores
),
score_totals AS (
    SELECT round_participant_id,
           count(*) AS submitted_score_count,
           count(*) FILTER (WHERE is_valid) AS valid_score_count,
           count(*) FILTER (WHERE NOT is_valid) AS invalid_score_count,
           count(DISTINCT judge_user_id) FILTER (WHERE is_valid) AS scoring_judge_count,
           avg(score_value) FILTER (WHERE is_valid) AS average_score
    FROM classified_scores
    GROUP BY round_participant_id
)
SELECT r.event_id, p.round_id, p.id AS round_participant_id, p.lapak_id, p.bird_id,
       COALESCE(s.submitted_score_count, 0) AS submitted_score_count,
       COALESCE(s.valid_score_count, 0) AS valid_score_count,
       COALESCE(s.invalid_score_count, 0) AS invalid_score_count,
       COALESCE(s.scoring_judge_count, 0) AS scoring_judge_count,
       s.average_score
FROM competition.round_participants AS p
JOIN competition.rounds AS r ON r.id = p.round_id
LEFT JOIN score_totals AS s ON s.round_participant_id = p.id
WITH DATA;

CREATE UNIQUE INDEX mv_round_score_summary_participant_uidx
    ON cloud_read.mv_round_score_summary (round_participant_id);
CREATE INDEX mv_round_score_summary_round_participant_idx
    ON cloud_read.mv_round_score_summary (round_id, round_participant_id);

COMMENT ON MATERIALIZED VIEW cloud_read.mv_round_score_summary IS
    'Administrative snapshot of available scores. Refresh after committed imports or corrections.';
