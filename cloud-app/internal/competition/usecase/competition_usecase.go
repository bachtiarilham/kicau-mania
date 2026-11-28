package usecase

import (
	"context"
	"strings"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/competition/contract"
	"kicau.cloud/control/internal/competition/model"
)

type CompetitionUsecase struct {
	tx      contract.TransactionManager
	events  contract.EventRepository
	rounds  contract.RoundRepository
	regs    contract.RegistrationRepository
	judging contract.JudgingRepository
}

func NewCompetitionUsecase(
	tx contract.TransactionManager,
	events contract.EventRepository,
	rounds contract.RoundRepository,
	regs contract.RegistrationRepository,
	judging contract.JudgingRepository,
) *CompetitionUsecase {
	return &CompetitionUsecase{
		tx:      tx,
		events:  events,
		rounds:  rounds,
		regs:    regs,
		judging: judging,
	}
}

// ---------------- Events ----------------

func (u *CompetitionUsecase) GetEvent(ctx context.Context, arenaID, id string) (*model.EventView, error) {
	return u.events.GetByID(ctx, arenaID, id)
}

func (u *CompetitionUsecase) ListEvents(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.EventView, error) {
	return u.events.List(ctx, arenaID, cursor, limit)
}

func (u *CompetitionUsecase) CreateEvent(ctx context.Context, arenaID string, req *model.CreateEventRequest) (*model.EventView, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.EventDate) == "" {
		return nil, apperror.Invalid("Judul dan tanggal event wajib diisi")
	}
	if req.Status == "" {
		req.Status = "SCHEDULED"
	}
	return u.events.Create(ctx, arenaID, req)
}

func (u *CompetitionUsecase) UpdateEvent(ctx context.Context, arenaID, id string, req *model.CreateEventRequest) (*model.EventView, error) {
	return u.events.Update(ctx, arenaID, id, req)
}

func (u *CompetitionUsecase) DeleteEvent(ctx context.Context, arenaID, id string) error {
	return u.events.Delete(ctx, arenaID, id)
}

// ---------------- Rounds ----------------

func (u *CompetitionUsecase) GetRound(ctx context.Context, arenaID, id string) (*model.RoundView, error) {
	return u.rounds.GetByID(ctx, arenaID, id)
}

func (u *CompetitionUsecase) ListRounds(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.RoundView, error) {
	return u.rounds.List(ctx, arenaID, cursor, limit)
}

func (u *CompetitionUsecase) CreateRound(ctx context.Context, arenaID string, req *model.CreateRoundRequest) (*model.RoundView, error) {
	if strings.TrimSpace(req.Title) == "" || req.RoundNumber < 1 || req.EventID == "" || req.BirdSpeciesID == "" {
		return nil, apperror.Invalid("Judul, nomor ronde, event, dan jenis burung wajib diisi")
	}
	return u.rounds.Create(ctx, arenaID, req)
}

func (u *CompetitionUsecase) UpdateRound(ctx context.Context, arenaID, id string, req *model.CreateRoundRequest) (*model.RoundView, error) {
	return u.rounds.Update(ctx, arenaID, id, req)
}

func (u *CompetitionUsecase) DeleteRound(ctx context.Context, arenaID, id string) error {
	return u.rounds.Delete(ctx, arenaID, id)
}

func (u *CompetitionUsecase) RoundAction(ctx context.Context, arenaID, id, action, expectedStatus string) (*model.RoundView, error) {
	newStatus := ""
	switch action {
	case "start":
		newStatus = "RUNNING"
	case "begin-judging":
		newStatus = "JUDGING"
	case "close":
		newStatus = "CLOSED"
	case "cancel":
		newStatus = "CANCELLED"
	default:
		return nil, apperror.Invalid("Aksi ronde tidak valid")
	}

	var updated *model.RoundView
	err := u.tx.Do(ctx, func(txCtx context.Context) error {
		r, err := u.rounds.Lock(txCtx, arenaID, id)
		if err != nil {
			return err
		}
		if expectedStatus != "" && r.Status != expectedStatus {
			return apperror.ErrConflict
		}
		res, err := u.rounds.SetStatus(txCtx, arenaID, id, r.Status, newStatus)
		if err != nil {
			return err
		}
		updated = res
		return nil
	})
	return updated, err
}

// ---------------- Registrations ----------------

func (u *CompetitionUsecase) GetRegistration(ctx context.Context, arenaID, id string) (*model.RegistrationView, error) {
	return u.regs.GetByID(ctx, arenaID, id)
}

func (u *CompetitionUsecase) ListRegistrations(ctx context.Context, arenaID string, roundID, cursor *string, limit int) ([]model.RegistrationView, error) {
	return u.regs.List(ctx, arenaID, roundID, cursor, limit)
}

func (u *CompetitionUsecase) CreateRegistration(ctx context.Context, arenaID string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error) {
	if req.UserID == "" || req.RoundID == "" || req.BirdID == "" {
		return nil, apperror.Invalid("Data pendaftaran tidak lengkap")
	}
	return u.regs.Create(ctx, arenaID, req)
}

func (u *CompetitionUsecase) UpdateRegistration(ctx context.Context, arenaID, id string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error) {
	return u.regs.Update(ctx, arenaID, id, req)
}

func (u *CompetitionUsecase) TicketAction(ctx context.Context, arenaID, id, action, reason string) (*model.RegistrationView, error) {
	switch action {
	case "verify":
		return u.regs.VerifyPayment(ctx, arenaID, id)
	case "reject":
		return u.regs.RejectPayment(ctx, arenaID, id)
	case "cancel":
		return u.regs.Cancel(ctx, arenaID, id)
	case "disqualify":
		if strings.TrimSpace(reason) == "" {
			return nil, apperror.Invalid("Alasan diskualifikasi wajib diisi")
		}
		return u.regs.Disqualify(ctx, arenaID, id, reason)
	case "requalify":
		return u.regs.Requalify(ctx, arenaID, id)
	default:
		return nil, apperror.Invalid("Aksi tiket tidak valid")
	}
}

func (u *CompetitionUsecase) UploadProof(ctx context.Context, arenaID, id string, bytes []byte, mime string) error {
	return u.regs.SetProof(ctx, arenaID, id, bytes, mime)
}

func (u *CompetitionUsecase) ReadProof(ctx context.Context, arenaID, id string) ([]byte, string, error) {
	return u.regs.ReadProof(ctx, arenaID, id)
}

func (u *CompetitionUsecase) RoundParticipants(ctx context.Context, arenaID, roundID string) ([]model.RegistrationView, error) {
	return u.regs.RoundParticipants(ctx, arenaID, roundID)
}

// ---------------- Judging ----------------

func (u *CompetitionUsecase) RoundJudges(ctx context.Context, arenaID, roundID string) ([]model.RoundJudgeView, error) {
	return u.judging.RoundJudges(ctx, arenaID, roundID)
}

func (u *CompetitionUsecase) AssignJudge(ctx context.Context, arenaID, roundID, judgeUserID string, remove bool) error {
	if remove {
		return u.judging.RemoveJudge(ctx, arenaID, roundID, judgeUserID)
	}
	return u.judging.AssignJudge(ctx, arenaID, roundID, judgeUserID)
}

func (u *CompetitionUsecase) SaveScores(ctx context.Context, arenaID, roundID, registrationID, judgeUserID string, scores []model.ScoreItemRequest) error {
	return u.tx.Do(ctx, func(txCtx context.Context) error {
		for _, s := range scores {
			if err := u.judging.SaveScore(txCtx, arenaID, roundID, registrationID, judgeUserID, s.CriteriaID, s.ScoreValue); err != nil {
				return err
			}
		}
		return nil
	})
}

func (u *CompetitionUsecase) ReadScores(ctx context.Context, arenaID, roundID, registrationID string) ([]model.ScoreView, error) {
	return u.judging.ReadScores(ctx, arenaID, roundID, registrationID)
}

func (u *CompetitionUsecase) RoundRecap(ctx context.Context, arenaID, roundID string) ([]model.ScoreSummaryView, error) {
	return u.judging.RoundRecap(ctx, arenaID, roundID)
}
