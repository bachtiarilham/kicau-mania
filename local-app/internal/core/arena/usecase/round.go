package usecase

// This file owns round transitions and recovery after server restarts.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

func (u *UseCase) Transition(ctx context.Context, eventID, roundID, action, reason string) error {
	u.ops.Lock()
	defer u.ops.Unlock()
	if action == "start" || action == "resume" {
		events, err := u.repo.Events(ctx)
		if err != nil {
			return err
		}
		for _, e := range events {
			for _, r := range e.Bundle.Rounds {
				if r.State == model.Active {
					return conflict("Masih ada ronde aktif. Selesaikan atau batalkan terlebih dahulu.")
				}
			}
		}
	}
	err := u.repo.Update(ctx, eventID, func(e *model.Event) ([]exchange.Change, error) {
		if e.Bundle.ID == "" {
			return nil, apperror.ErrNotFound
		}
		var round *model.Round
		for i := range e.Bundle.Rounds {
			if e.Bundle.Rounds[i].ID == roundID {
				round = &e.Bundle.Rounds[i]
				break
			}
		}
		if round == nil {
			return nil, apperror.ErrNotFound
		}
		switch action {
		case "start":
			if round.State != model.Prepared {
				return nil, conflict("Hanya ronde PREPARED yang dapat dimulai.")
			}
			round.State = model.Active
		case "resume":
			if round.State != model.Interrupted {
				return nil, conflict("Ronde tidak terinterupsi.")
			}
			round.State = model.Active
		case "close":
			if round.State != model.Active && round.State != model.Interrupted {
				return nil, conflict("Ronde tidak dapat ditutup.")
			}
			for _, result := range model.Results(*e, *round) {
				if result.Count != result.Required {
					return nil, conflict("Penilaian wajib belum lengkap. Ronde hanya dapat dilanjutkan atau dibatalkan.")
				}
			}
			round.State = model.Closed
		case "cancel":
			if round.State == model.Closed || round.State == model.Cancelled {
				return nil, conflict("Ronde sudah final.")
			}
			if strings.TrimSpace(reason) == "" || len(reason) > 1000 {
				return nil, invalid("Alasan pembatalan wajib diisi, maksimal 1000 karakter.")
			}
			round.State = model.Cancelled
			round.Reason = reason
		default:
			return nil, invalid("Transisi ronde tidak dikenal.")
		}
		audit := model.Audit{ID: operationID(), RoundID: roundID, Actor: "operator", Action: action, Reason: reason, At: time.Now().UTC()}
		e.Audit = append(e.Audit, audit)
		v, err := change(e, "round."+action, audit)
		return []exchange.Change{v}, err
	})
	if err == nil && action == "close" && u.opts.BackupDir != "" {
		_, _ = u.Backup(ctx)
	}
	return err
}

// Recover is completed before HTTP accepts mutations. It never resumes judging.
func (u *UseCase) Recover(ctx context.Context) error {
	events, err := u.repo.Events(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Bundle.ArenaID != u.opts.ArenaID || event.Bundle.EngineID != u.opts.EngineID {
			return fmt.Errorf("persisted event identity does not match ARENA_ID/ENGINE_ID")
		}
		err = u.repo.Update(ctx, event.Bundle.ID, func(e *model.Event) ([]exchange.Change, error) {
			changes := []exchange.Change{}
			for i := range e.Bundle.Rounds {
				r := &e.Bundle.Rounds[i]
				if r.State != model.Active {
					continue
				}
				r.State = model.Interrupted
				a := model.Audit{ID: operationID(), RoundID: r.ID, Actor: "system", Action: "interrupted", Reason: "Server restarted", At: time.Now().UTC()}
				e.Audit = append(e.Audit, a)
				v, err := change(e, "round.interrupted", a)
				if err != nil {
					return nil, err
				}
				changes = append(changes, v)
			}
			return changes, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
