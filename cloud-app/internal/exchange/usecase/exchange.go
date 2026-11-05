package usecase

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"kicau.cloud/control/internal/exchange/model"
)

type Repository interface {
	SavePackage(context.Context, model.Package, []byte) error
	Package(context.Context, string, string) ([]byte, error)
	Receive(context.Context, string, []model.Change) error
	Enqueue(context.Context, model.Command) error
	Pending(context.Context, string) ([]model.Command, error)
	Results(context.Context, string, []model.Result) error
	Command(context.Context, string, string) (model.Command, *model.Result, error)
}

func (u *UseCase) CommandState(ctx context.Context, token, engine, id string) (model.CommandState, error) {
	provided := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(provided[:], u.adminHash[:]) != 1 {
		return model.CommandState{}, model.ErrUnauthorized
	}
	command, result, err := u.repo.Command(ctx, engine, id)
	state := "PENDING"
	if result != nil {
		state = result.State
	}
	return model.CommandState{Command: command, Result: result, State: state}, err
}

func (u *UseCase) Prepare(ctx context.Context, token string, body json.RawMessage) error {
	provided := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(provided[:], u.adminHash[:]) != 1 {
		return model.ErrUnauthorized
	}
	var metadata model.Package
	if json.Unmarshal(body, &metadata) != nil || !identifier.MatchString(metadata.ID) || metadata.Version < 1 {
		return model.ErrInvalid
	}
	engine, ok := u.engines[metadata.EngineID]
	if !ok || engine.ArenaID != metadata.ArenaID {
		return model.ErrInvalid
	}
	return u.repo.SavePackage(ctx, metadata, body)
}
func (u *UseCase) Package(ctx context.Context, engine, token, id string) (json.RawMessage, error) {
	if _, err := u.authenticate(engine, token); err != nil {
		return nil, err
	}
	if !identifier.MatchString(id) {
		return nil, model.ErrInvalid
	}
	body, err := u.repo.Package(ctx, engine, id)
	return json.RawMessage(body), err
}

type UseCase struct {
	repo      Repository
	engines   map[string]model.Engine
	adminHash [32]byte
}

var identifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func New(repo Repository, engines []model.Engine, admin string) (*UseCase, error) {
	u := &UseCase{repo: repo, engines: map[string]model.Engine{}, adminHash: sha256.Sum256([]byte(admin))}
	if len(admin) < 32 || strings.HasPrefix(admin, "REPLACE_") {
		return nil, model.ErrInvalid
	}
	for _, engine := range engines {
		if !identifier.MatchString(engine.EngineID) || !identifier.MatchString(engine.ArenaID) || len(engine.Token) < 32 || strings.HasPrefix(engine.Token, "REPLACE_") {
			return nil, model.ErrInvalid
		}
		if _, exists := u.engines[engine.EngineID]; exists {
			return nil, model.ErrInvalid
		}
		u.engines[engine.EngineID] = engine
	}
	if len(u.engines) == 0 {
		return nil, model.ErrInvalid
	}
	return u, nil
}
func (u *UseCase) authenticate(engine, token string) (model.Engine, error) {
	e, ok := u.engines[engine]
	expected, provided := sha256.Sum256([]byte(e.Token)), sha256.Sum256([]byte(token))
	if !ok || subtle.ConstantTimeCompare(expected[:], provided[:]) != 1 {
		return e, model.ErrUnauthorized
	}
	return e, nil
}
func (u *UseCase) Receive(ctx context.Context, engine, token string, changes []model.Change) ([]string, error) {
	e, err := u.authenticate(engine, token)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 || len(changes) > 100 {
		return nil, model.ErrInvalid
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, c := range changes {
		if !identifier.MatchString(c.ID) || !identifier.MatchString(c.EventID) || seen[c.ID] || c.ArenaID != e.ArenaID || c.EngineID != e.EngineID || c.At.IsZero() || !json.Valid(c.Payload) {
			return nil, model.ErrInvalid
		}
		seen[c.ID] = true
		ids = append(ids, c.ID)
	}
	if err := u.repo.Receive(ctx, engine, changes); err != nil {
		return nil, err
	}
	return ids, nil
}
func (u *UseCase) Enqueue(ctx context.Context, token string, c model.Command) error {
	provided := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(provided[:], u.adminHash[:]) != 1 {
		return model.ErrUnauthorized
	}
	e, ok := u.engines[c.EngineID]
	if !ok || e.ArenaID != c.ArenaID || !identifier.MatchString(c.ID) || !json.Valid(c.Payload) || !c.ExpiresAt.After(time.Now()) || c.ExpiresAt.After(time.Now().Add(15*time.Minute)) {
		return model.ErrInvalid
	}
	switch c.Kind {
	case "restream.start", "restream.stop", "recording.start", "recording.stop":
	default:
		return model.ErrInvalid
	}
	return u.repo.Enqueue(ctx, c)
}
func (u *UseCase) Commands(ctx context.Context, engine, token string) ([]model.Command, error) {
	if _, err := u.authenticate(engine, token); err != nil {
		return nil, err
	}
	return u.repo.Pending(ctx, engine)
}
func (u *UseCase) Results(ctx context.Context, engine, token string, results []model.Result) ([]string, error) {
	if _, err := u.authenticate(engine, token); err != nil {
		return nil, err
	}
	if len(results) == 0 || len(results) > 100 {
		return nil, model.ErrInvalid
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, r := range results {
		if !identifier.MatchString(r.ID) || r.At.IsZero() || seen[r.ID] {
			return nil, model.ErrInvalid
		}
		switch r.State {
		case "SUCCEEDED", "FAILED", "EXPIRED":
		default:
			return nil, model.ErrInvalid
		}
		seen[r.ID] = true
		ids = append(ids, r.ID)
	}
	if err := u.repo.Results(ctx, engine, results); err != nil {
		return nil, err
	}
	return ids, nil
}
