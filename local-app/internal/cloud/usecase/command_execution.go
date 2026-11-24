package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"kicau.local/edge/internal/apperror"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	session "kicau.local/edge/internal/recording/model"
)

type Sessions interface {
	StartSession(context.Context, session.StartRequest) (session.Session, error)
	StopSession(context.Context, string) (session.Session, error)
	Get(context.Context, string) (session.Session, error)
}
type CommandExecutor struct{ sessions Sessions }

func NewCommandExecutor(sessions Sessions) *CommandExecutor {
	return &CommandExecutor{sessions: sessions}
}
func (u *CommandExecutor) Execute(ctx context.Context, command cloudmodel.Command) error {
	var req session.StartRequest
	if json.Unmarshal(command.Payload, &req) != nil || req.ID == "" {
		return apperror.ErrInvalid
	}
	switch command.Kind {
	case "media.session.start":
		_, err := u.sessions.StartSession(ctx, req)
		return err
	case "media.session.stop":
		_, err := u.sessions.StopSession(ctx, req.ID)
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	default:
		return apperror.ErrForbidden
	}
}
func (u *CommandExecutor) Reconcile(ctx context.Context, command cloudmodel.Command) error {
	if command.Kind == "media.session.stop" {
		return u.Execute(ctx, command)
	}
	if command.Kind != "media.session.start" {
		return apperror.ErrForbidden
	}
	var req session.StartRequest
	if json.Unmarshal(command.Payload, &req) != nil {
		return apperror.ErrInvalid
	}
	s, err := u.sessions.Get(ctx, req.ID)
	if err != nil {
		return err
	}
	a, _ := json.Marshal(req)
	b, _ := json.Marshal(s.StartRequest)
	if string(a) != string(b) || session.Terminal(s.State) || s.State == "STOPPING" || s.State == "FINALIZING" {
		return apperror.Public(apperror.ErrConflict, "Interrupted media session requires operator review.")
	}
	return nil
}
