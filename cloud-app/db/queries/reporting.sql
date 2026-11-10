-- name: ReportTryLock
SELECT jsonb_build_object('locked',pg_try_advisory_xact_lock($1::bigint),'snapshot_at',clock_timestamp());
-- name: RefreshRoundSummary
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
-- name: RefreshArenaDashboard
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_arena_dashboard;
-- name: ReportMark
UPDATE cloud_read.refresh_status SET snapshot_at=$2::timestamptz,refreshed_at=clock_timestamp() WHERE view_name=$1;
-- name: ReportStatus
SELECT to_jsonb(v) FROM cloud_read.refresh_status v WHERE view_name=$1;
-- name: ReportDashboard
SELECT to_jsonb(v) FROM cloud_read.mv_arena_dashboard v WHERE arena_id=$1::uuid;
-- name: ReportRound
SELECT to_jsonb(v) FROM cloud_read.mv_round_score_summary v WHERE arena_id=$1::uuid AND round_id=$2::uuid ORDER BY registration_item_id;

-- name: ReportSnapshotDashboard
SELECT jsonb_build_object('snapshot_at',s.snapshot_at,'refreshed_at',s.refreshed_at,'items',coalesce((SELECT jsonb_agg(to_jsonb(v) ORDER BY v.arena_id) FROM cloud_read.mv_arena_dashboard v WHERE v.arena_id=$1::uuid),'[]'::jsonb)) FROM cloud_read.refresh_status s WHERE view_name='mv_arena_dashboard';
-- name: ReportSnapshotRound
SELECT jsonb_build_object('snapshot_at',s.snapshot_at,'refreshed_at',s.refreshed_at,'items',coalesce((SELECT jsonb_agg(to_jsonb(v) ORDER BY v.registration_item_id) FROM cloud_read.mv_round_score_summary v WHERE v.arena_id=$1::uuid AND v.round_id=$2::uuid),'[]'::jsonb)) FROM cloud_read.refresh_status s WHERE view_name='mv_round_score_summary';
