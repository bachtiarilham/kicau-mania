package usecase

// This file builds event views, score history, and operational status for callers.

import (
	"context"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/mediaengine/media"
)

type CatalogItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type RoundView struct {
	model.Round
	Scores    []model.Revision    `json:"scores"`
	Results   []model.ScoreResult `json:"results,omitempty"`
	Completed int                 `json:"completed"`
	Required  int                 `json:"required"`
	Playback  map[string]string   `json:"playback"`
}

type View struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Version  int               `json:"version"`
	JudgeID  string            `json:"judge_id,omitempty"`
	Criteria []model.Criterion `json:"criteria"`
	Rounds   []RoundView       `json:"rounds"`
	Audit    []model.Audit     `json:"audit,omitempty"`
}

type RoundState struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type Status struct {
	ArenaID          string       `json:"arena_id"`
	EngineID         string       `json:"engine_id"`
	Media            media.Status `json:"media"`
	Pending          int          `json:"pending_changes"`
	BackupConfigured bool         `json:"backup_configured"`
	LastBackup       time.Time    `json:"last_backup"`
	BackupFailed     bool         `json:"backup_failed"`
}

func (u *UseCase) Catalog(ctx context.Context) ([]CatalogItem, error) {
	events, err := u.repo.Events(ctx)
	if err != nil {
		return nil, err
	}
	out := []CatalogItem{}
	for _, e := range events {
		out = append(out, CatalogItem{e.Bundle.ID, e.Bundle.Name})
	}
	return out, nil
}

func (u *UseCase) view(e model.Event, judgeID string) View {
	v := View{ID: e.Bundle.ID, Name: e.Bundle.Name, Version: e.Bundle.Version, JudgeID: judgeID, Criteria: e.Bundle.Criteria, Rounds: []RoundView{}}
	if judgeID == "" {
		v.Audit = e.Audit
	}
	for _, r := range e.Bundle.Rounds {
		if judgeID != "" && !contains(r.JudgeIDs, judgeID) {
			continue
		}
		rv := RoundView{Round: r, Scores: []model.Revision{}, Required: 24 * len(e.Bundle.Criteria), Playback: map[string]string{}}
		if judgeID == "" {
			rv.Required *= len(r.JudgeIDs)
			rv.Results = model.Results(e, r)
		}
		for _, rev := range model.Latest(e, r.ID) {
			if judgeID == "" || rev.JudgeID == judgeID {
				rev.Fingerprint = ""
				rv.Scores = append(rv.Scores, rev)
				rv.Completed += len(rev.Scores)
			}
		}
		for _, p := range r.Participants {
			if u.playback != nil {
				rv.Playback[p.ChannelID] = u.playback.WHEP(p.ChannelID)
			}
		}
		v.Rounds = append(v.Rounds, rv)
	}
	return v
}

func (u *UseCase) JudgeView(ctx context.Context, token string) (View, error) {
	s, err := u.session(ctx, token)
	if err != nil {
		return View{}, err
	}
	e, err := u.repo.JudgeEvent(ctx, s.EventID, s.JudgeID)
	if err != nil {
		return View{}, err
	}
	if e.Bundle.Version != s.Version {
		return View{}, apperror.ErrUnauthorized
	}
	return u.view(e, s.JudgeID), nil
}

func (u *UseCase) JudgeStates(ctx context.Context, token string) ([]RoundState, error) {
	s, err := u.session(ctx, token)
	if err != nil {
		return nil, err
	}
	e, err := u.repo.Metadata(ctx, s.EventID)
	if err != nil {
		return nil, err
	}
	if e.Bundle.Version != s.Version {
		return nil, apperror.ErrUnauthorized
	}
	out := []RoundState{}
	for _, r := range e.Bundle.Rounds {
		if contains(r.JudgeIDs, s.JudgeID) {
			out = append(out, RoundState{ID: r.ID, State: r.State})
		}
	}
	return out, nil
}

func (u *UseCase) OperatorView(ctx context.Context, eventID string) (View, error) {
	e, err := u.repo.Event(ctx, eventID)
	if err != nil {
		return View{}, err
	}
	return u.view(e, ""), nil
}

func (u *UseCase) History(ctx context.Context, eventID string) ([]model.Revision, error) {
	e, err := u.repo.Event(ctx, eventID)
	if err != nil {
		return nil, err
	}
	for i := range e.Revisions {
		e.Revisions[i].Fingerprint = ""
	}
	return e.Revisions, nil
}

func (u *UseCase) Pending(ctx context.Context) (int, error) { return u.repo.PendingCount(ctx) }

func (u *UseCase) Status(ctx context.Context) (Status, error) {
	n, err := u.repo.PendingCount(ctx)
	s := Status{ArenaID: u.opts.ArenaID, EngineID: u.opts.EngineID, Pending: n, BackupConfigured: u.opts.BackupDir != "", Media: media.Status{State: "UNAVAILABLE"}}
	if provider, ok := u.playback.(interface{ MediaStatus() media.Status }); ok {
		s.Media = provider.MediaStatus()
	}
	u.healthMu.Lock()
	s.LastBackup = u.lastBackup
	s.BackupFailed = u.backupFailed
	u.healthMu.Unlock()
	return s, err
}
