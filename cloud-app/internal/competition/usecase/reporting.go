package usecase

import (
	"context"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/infra/logger"
)

type ReportRepository interface {
	ReportSnapshot(context.Context, string, string) (model.Record, error)
	ReportStatus(context.Context, string) (model.Record, error)
	ReportRows(context.Context, string, string) ([]model.Record, error)
	ReportLock(context.Context, int64) (model.Record, error)
	ReportRefresh(context.Context, string) error
	ReportMark(context.Context, string, string) error
}
type Reports struct {
	repo ReportRepository
	auth Authority
	tx   Transactions
}

func NewReports(r ReportRepository, a Authority, tx Transactions) *Reports { return &Reports{r, a, tx} }
func (u *Reports) Get(ctx context.Context, actor, arena, round string) (model.Record, error) {
	if !model.UUID(arena) || round != "" && !model.UUID(round) {
		return nil, apperror.ErrInvalid
	}
	if e := u.auth.Require(ctx, actor, arena, "admin_arena", "operator"); e != nil {
		return nil, e
	}
	return u.repo.ReportSnapshot(ctx, arena, round)
}
func (u *Reports) Refresh(ctx context.Context) {
	for i, name := range []string{"mv_round_score_summary", "mv_arena_dashboard"} {
		if ctx.Err() != nil {
			return
		}
		err := u.tx.Do(ctx, func(ctx context.Context) error {
			lock, e := u.repo.ReportLock(ctx, int64(753120+i))
			if e != nil {
				return e
			}
			if !model.Flag(lock, "locked") {
				return nil
			}
			if e = u.repo.ReportRefresh(ctx, name); e != nil {
				return e
			}
			return u.repo.ReportMark(ctx, name, model.Text(lock, "snapshot_at"))
		})
		if err != nil && ctx.Err() == nil {
			logger.Warn("Report refresh failed; previous snapshot retained")
		}
	}
}
