package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

type CommandRepository interface {
	Command(context.Context, string) (exchange.Command, *exchange.CommandResult, error)
	SaveCommand(context.Context, exchange.Command) error
	CompleteCommand(context.Context, exchange.CommandResult) error
	CommandResults(context.Context) ([]exchange.CommandResult, error)
	AcknowledgeCommands(context.Context, []string) error
}
type CommandCloud interface {
	Commands(context.Context) ([]exchange.Command, error)
	Report(context.Context, []exchange.CommandResult) ([]string, error)
}
type CommandRunner interface {
	Execute(context.Context, exchange.Command) error
}
type Commands struct {
	repo          CommandRepository
	cloud         CommandCloud
	runner        CommandRunner
	arena, engine string
	mu            sync.Mutex
}

func NewCommands(repo CommandRepository, cloud CommandCloud, runner CommandRunner, arena, engine string) *Commands {
	return &Commands{repo: repo, cloud: cloud, runner: runner, arena: arena, engine: engine}
}
func (u *Commands) Poll(ctx context.Context) error {
	if u.cloud == nil {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	commands, err := u.cloud.Commands(ctx)
	if err != nil {
		return err
	}
	for _, command := range commands {
		if !identifier.Valid(command.ID) || command.ArenaID != u.arena || command.EngineID != u.engine || command.ExpiresAt.IsZero() {
			return fmt.Errorf("command identity mismatch")
		}
		stored, result, err := u.repo.Command(ctx, command.ID)
		if err != nil && !errors.Is(err, apperror.ErrNotFound) {
			return err
		}
		if err == nil {
			old, _ := json.Marshal(stored)
			incoming, _ := json.Marshal(command)
			if string(old) != string(incoming) {
				return fmt.Errorf("command ID reused")
			}
			if result != nil {
				continue
			}
		}
		if err := u.repo.SaveCommand(ctx, command); err != nil {
			return err
		}
		state := "SUCCEEDED"
		if !time.Now().Before(command.ExpiresAt) {
			state = "EXPIRED"
		} else if err := u.runner.Execute(ctx, command); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			state = "FAILED"
		}
		if err := u.repo.CompleteCommand(ctx, exchange.CommandResult{ID: command.ID, State: state, At: time.Now().UTC()}); err != nil {
			return err
		}
	}
	results, err := u.repo.CommandResults(ctx)
	if err != nil || len(results) == 0 {
		return err
	}
	ids, err := u.cloud.Report(ctx, results)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, r := range results {
		allowed[r.ID] = true
	}
	for _, id := range ids {
		if !allowed[id] {
			return fmt.Errorf("invalid command acknowledgment")
		}
		delete(allowed, id)
	}
	return u.repo.AcknowledgeCommands(ctx, ids)
}
