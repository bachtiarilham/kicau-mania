package usecase

// This file accepts score submissions and revisions with transactional idempotency.

import (
	"context"
	"encoding/json"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/identifier"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

func (u *UseCase) Submit(ctx context.Context, token, roundID string, submission model.Submission) (model.Revision, error) {
	var accepted model.Revision
	s, err := u.session(ctx, token)
	if err != nil {
		return accepted, err
	}
	// Bind browser queues to their original account even if another tab changes
	// the shared session cookie while an offline submission is being retried.
	if (submission.EventID != "" && submission.EventID != s.EventID) || (submission.JudgeID != "" && submission.JudgeID != s.JudgeID) {
		return accepted, apperror.ErrForbidden
	}
	if !identifier.Valid(submission.OperationID) || submission.ExpectedRevision < 0 {
		return accepted, invalid("operation_id dan expected_revision tidak valid.")
	}
	encoded, _ := json.Marshal(struct {
		Round, Judge string
		Submission   model.Submission
	}{roundID, s.JudgeID, submission})
	fingerprint := hash(encoded)
	err = u.repo.UpdateScores(ctx, s.EventID, roundID, submission.OperationID, func(e *model.Event) ([]exchange.Change, error) {
		if e.Bundle.ID == "" {
			return nil, apperror.ErrNotFound
		}
		if e.Bundle.Version != s.Version {
			return nil, apperror.ErrUnauthorized
		}
		for _, rev := range e.Revisions {
			if rev.OperationID == submission.OperationID {
				if rev.Fingerprint != fingerprint {
					return nil, conflict("operation_id sudah dipakai untuk request berbeda.")
				}
				accepted = rev
				return nil, nil
			}
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
		if !contains(round.JudgeIDs, s.JudgeID) {
			return nil, apperror.ErrForbidden
		}
		if round.State != model.Active {
			return nil, conflict("Ronde tidak aktif. Draft tetap di perangkat; hubungi operator.")
		}
		found := false
		for _, p := range round.Participants {
			if p.ID == submission.ParticipantID {
				found = true
			}
		}
		if !found {
			return nil, apperror.ErrNotFound
		}
		if len(submission.Scores) != len(e.Bundle.Criteria) {
			return nil, invalid("Seluruh kriteria wajib diisi.")
		}
		for _, c := range e.Bundle.Criteria {
			v, ok := submission.Scores[c.ID]
			if !ok || v < 1 || v > 10 {
				return nil, invalid("Nilai setiap kriteria wajib bilangan bulat 1–10.")
			}
		}
		previous := model.Latest(*e, roundID)[s.JudgeID+":"+submission.ParticipantID]
		if previous.Number != submission.ExpectedRevision {
			return nil, conflict("Nilai sudah direvisi dari perangkat lain. Muat ulang sebelum mengirim.")
		}
		accepted = model.Revision{OperationID: submission.OperationID, Fingerprint: fingerprint, RoundID: roundID, JudgeID: s.JudgeID, ParticipantID: submission.ParticipantID, Number: previous.Number + 1, Scores: submission.Scores, At: time.Now().UTC()}
		e.Revisions = append(e.Revisions, accepted)
		v, err := change(e, "score.revised", accepted)
		return []exchange.Change{v}, err
	})
	accepted.Fingerprint = ""
	return accepted, err
}
