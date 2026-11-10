package repository

import (
	"context"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
)

func (r *Repository) ReportStatus(ctx context.Context, name string) (model.Record, error) {
	return r.db.One(ctx, "ReportStatus", name)
}
func (r *Repository) ReportRows(ctx context.Context, arena, round string) ([]model.Record, error) {
	if round == "" {
		return r.db.Many(ctx, "ReportDashboard", arena)
	}
	return r.db.Many(ctx, "ReportRound", arena, round)
}
func (r *Repository) ReportLock(ctx context.Context, key int64) (model.Record, error) {
	return r.db.One(ctx, "ReportTryLock", key)
}
func (r *Repository) ReportRefresh(ctx context.Context, name string) error {
	switch name {
	case "mv_round_score_summary":
		return r.db.Exec(ctx, "RefreshRoundSummary")
	case "mv_arena_dashboard":
		return r.db.Exec(ctx, "RefreshArenaDashboard")
	}
	return apperror.ErrInvalid
}
func (r *Repository) ReportMark(ctx context.Context, name, snapshot string) error {
	return r.db.Exec(ctx, "ReportMark", name, snapshot)
}

func (r *Repository) ReportSnapshot(ctx context.Context, arena, round string) (model.Record, error) {
	if round == "" {
		return r.db.One(ctx, "ReportSnapshotDashboard", arena)
	}
	return r.db.One(ctx, "ReportSnapshotRound", arena, round)
}
