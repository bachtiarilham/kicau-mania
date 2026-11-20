package contract

import (
	"context"

	"kicau.cloud/control/internal/competition/model"
)

type TransactionManager interface {
	Do(ctx context.Context, f func(context.Context) error) error
}

type EventRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.EventView, error)
	List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.EventView, error)
	Create(ctx context.Context, arenaID string, req *model.CreateEventRequest) (*model.EventView, error)
	Update(ctx context.Context, arenaID, id string, req *model.CreateEventRequest) (*model.EventView, error)
	Delete(ctx context.Context, arenaID, id string) error
}

type RoundRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.RoundView, error)
	List(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.RoundView, error)
	Create(ctx context.Context, arenaID string, req *model.CreateRoundRequest) (*model.RoundView, error)
	Update(ctx context.Context, arenaID, id string, req *model.CreateRoundRequest) (*model.RoundView, error)
	Delete(ctx context.Context, arenaID, id string) error
	Lock(ctx context.Context, arenaID, id string) (*model.RoundView, error)
	SetStatus(ctx context.Context, arenaID, id, expectedStatus, newStatus string) (*model.RoundView, error)
}

type RegistrationRepository interface {
	GetByID(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	List(ctx context.Context, arenaID string, roundID, cursor *string, limit int) ([]model.RegistrationView, error)
	Create(ctx context.Context, arenaID string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error)
	Update(ctx context.Context, arenaID, id string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error)
	Lock(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	VerifyPayment(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	RejectPayment(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	Cancel(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	Disqualify(ctx context.Context, arenaID, id, reason string) (*model.RegistrationView, error)
	Requalify(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	SetProof(ctx context.Context, arenaID, id string, imageBytes []byte, mimeType string) error
	ReadProof(ctx context.Context, arenaID, id string) ([]byte, string, error)
	RoundParticipants(ctx context.Context, arenaID, roundID string) ([]model.RegistrationView, error)
}

type JudgingRepository interface {
	RoundJudges(ctx context.Context, arenaID, roundID string) ([]model.RoundJudgeView, error)
	AssignJudge(ctx context.Context, arenaID, roundID, judgeUserID string) error
	RemoveJudge(ctx context.Context, arenaID, roundID, judgeUserID string) error
	SaveScore(ctx context.Context, arenaID, roundID, registrationID, judgeUserID, criteriaID string, scoreValue float64) error
	ReadScores(ctx context.Context, arenaID, roundID, registrationID string) ([]model.ScoreView, error)
	RoundRecap(ctx context.Context, arenaID, roundID string) ([]model.ScoreSummaryView, error)
}
