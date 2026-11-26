-- ERD 2026-11-15. Parameters are bound by pgx, never string interpolation.
-- UseCase validates authorization, semantics and transaction/lock order; see README.md.

-- name: ReportRoundSnapshot
-- params: arena_id bigint, round_id bigint
SELECT to_jsonb(v) FROM cloud_read.mv_round_score_summary v WHERE arena_id=$1::text::bigint AND round_id=$2::text::bigint ORDER BY average_score DESC NULLS LAST,registration_id;

-- name: ReportArenaDashboard
-- params: arena_id bigint
SELECT to_jsonb(v) FROM cloud_read.mv_arena_dashboard v WHERE arena_id=$1::text::bigint;

-- name: ReportRefreshStatus
-- params: view_name text
SELECT to_jsonb(v) FROM cloud_read.refresh_status v WHERE view_name=$1;

-- name: ReportRoundRefresh
-- params: none
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;

-- name: ReportDashboardRefresh
-- params: none
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_arena_dashboard;

-- name: ReportRefreshLock
-- params: lock_key bigint (753120 or 753121)
SELECT pg_advisory_xact_lock($1::bigint);

-- name: ReportMarkRefreshed
-- params: view_name text, snapshot_started timestamptz
UPDATE cloud_read.refresh_status SET snapshot_at=$2::timestamptz,refreshed_at=clock_timestamp() WHERE view_name=$1;
