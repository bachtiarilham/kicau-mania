// Package usecase owns offline event validation, authentication, scoring, and recovery.
package usecase

// This file defines dependency contracts, shared state, construction, and common helpers.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
)

type Repository interface {
	Event(context.Context, string) (model.Event, error)
	Metadata(context.Context, string) (model.Event, error)
	JudgeEvent(context.Context, string, string) (model.Event, error)
	Events(context.Context) ([]model.Event, error)
	Update(context.Context, string, func(*model.Event) ([]exchange.Change, error)) error
	UpdateScores(context.Context, string, string, string, func(*model.Event) ([]exchange.Change, error)) error
	SaveSession(context.Context, model.Session) error
	Session(context.Context, string) (model.Session, error)
	DeleteSession(context.Context, string) error
	PendingCount(context.Context) (int, error)
	Backup(context.Context, string) error
}

type Playback interface{ WHEP(string) string }

type PackageSource interface {
	Fetch(context.Context, string) ([]byte, error)
}

type Options struct {
	PackageSource                PackageSource
	ArenaID, EngineID, BackupDir string
	SessionTTL                   time.Duration
}

type UseCase struct {
	repo         Repository
	playback     Playback
	opts         Options
	ops          sync.Mutex
	loginMu      sync.Mutex
	attempts     map[string]attempt
	dummyHash    []byte
	healthMu     sync.Mutex
	lastBackup   time.Time
	backupFailed bool
}

func New(repo Repository, playback Playback, opts Options) *UseCase {
	if opts.SessionTTL == 0 {
		opts.SessionTTL = 24 * time.Hour
	}
	dummy, _ := bcrypt.GenerateFromPassword([]byte("invalid-event-account"), bcrypt.DefaultCost)
	return &UseCase{repo: repo, playback: playback, opts: opts, attempts: map[string]attempt{}, dummyHash: dummy}
}

func operationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("operating system entropy unavailable")
	}
	return hex.EncodeToString(b[:])
}

func hash(data []byte) string { b := sha256.Sum256(data); return hex.EncodeToString(b[:]) }

func invalid(message string) error { return apperror.Public(apperror.ErrInvalid, message) }

func conflict(message string) error { return apperror.Public(apperror.ErrConflict, message) }

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func change(e *model.Event, kind string, payload any) (exchange.Change, error) {
	data, err := json.Marshal(payload)
	return exchange.Change{ID: operationID(), ArenaID: e.Bundle.ArenaID, EngineID: e.Bundle.EngineID, EventID: e.Bundle.ID, Kind: kind, At: time.Now().UTC(), Payload: data}, err
}
