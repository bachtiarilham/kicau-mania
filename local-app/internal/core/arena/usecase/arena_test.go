package usecase_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	exchange "kicau.local/edge/internal/edgetocloud/exchange/model"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/arena/model"
	"kicau.local/edge/internal/core/arena/repository"
	"kicau.local/edge/internal/core/arena/usecase"
)

var ctx = context.Background()

func bundle(t *testing.T) model.Bundle {
	t.Helper()
	password, err := bcrypt.GenerateFromPassword([]byte("event-password"), 10)
	if err != nil {
		t.Fatal(err)
	}
	b := model.Bundle{ID: "event01", Name: "Event offline", Version: 1, ArenaID: "arena01", EngineID: "engine01", Judges: []model.Judge{{ID: "j1", Username: "juri1", Name: "Juri satu", PasswordHash: string(password)}, {ID: "j2", Username: "juri2", Name: "Juri dua", PasswordHash: string(password)}}, Criteria: []model.Criterion{{ID: "c1", Name: "Kriteria satu"}, {ID: "c2", Name: "Kriteria dua"}}, Rounds: []model.Round{{ID: "round01", Name: "Ronde satu", JudgeIDs: []string{"j1", "j2"}}}}
	for i := 1; i <= 24; i++ {
		b.Rounds[0].Participants = append(b.Rounds[0].Participants, model.Participant{ID: fmt.Sprintf("p%02d", i), Name: fmt.Sprintf("Peserta %d", i), Lapak: i, ChannelID: fmt.Sprintf("lapak%02d", i)})
	}
	return b
}
func open(t *testing.T, path string) (*repository.SQLite, *usecase.UseCase) {
	t.Helper()
	r, err := repository.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	u := usecase.New(r, nil, usecase.Options{ArenaID: "arena01", EngineID: "engine01"})
	return r, u
}
func prepare(t *testing.T) (*repository.SQLite, *usecase.UseCase, string, string) {
	t.Helper()
	r, u := open(t, filepath.Join(t.TempDir(), "arena.sqlite"))
	t.Cleanup(func() { r.Close() })
	if err := u.Prepare(ctx, bundle(t)); err != nil {
		t.Fatal(err)
	}
	if err := u.Transition(ctx, "event01", "round01", "start", ""); err != nil {
		t.Fatal(err)
	}
	a, _, err := u.Login(ctx, "event01", "juri1", "event-password", "client1")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := u.Login(ctx, "event01", "juri2", "event-password", "client2")
	if err != nil {
		t.Fatal(err)
	}
	return r, u, a, b
}
func submission(id, participant string, expected, value int) model.Submission {
	return model.Submission{OperationID: id, ParticipantID: participant, ExpectedRevision: expected, Scores: map[string]int{"c1": value, "c2": value}}
}

func TestQueuedScoresStayBoundToOriginalEventAndJudge(t *testing.T) {
	_, u, first, second := prepare(t)
	req := submission("queued-original", "p01", 0, 8)
	req.EventID, req.JudgeID = "event01", "j1"
	if _, err := u.Submit(ctx, second, "round01", req); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatal("a changed browser cookie attributed queued scores to another judge", err)
	}
	req.EventID = "another-event"
	if _, err := u.Submit(ctx, first, "round01", req); !errors.Is(err, apperror.ErrForbidden) {
		t.Fatal("cross-event queue accepted", err)
	}
	req.EventID = "event01"
	if _, err := u.Submit(ctx, first, "round01", req); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineScoringRevisionsCompletenessAndRoundLock(t *testing.T) {
	r, u, a, b := prepare(t)
	if err := u.Transition(ctx, "event01", "round01", "close", ""); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("incomplete round closed", err)
	}
	for j, token := range []string{a, b} {
		for i := 1; i <= 24; i++ {
			_, err := u.Submit(ctx, token, "round01", submission(fmt.Sprintf("score-%d-%d", j, i), fmt.Sprintf("p%02d", i), 0, 8-2*j))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	first := submission("revised-one", "p01", 1, 10)
	accepted, err := u.Submit(ctx, a, "round01", first)
	if err != nil || accepted.Number != 2 {
		t.Fatal(accepted, err)
	}
	if _, err := u.Submit(ctx, a, "round01", submission("stale-one", "p01", 1, 2)); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("stale revision overwrote current score", err)
	}
	judgeView, err := u.JudgeView(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if len(judgeView.Rounds[0].Scores) != 24 || len(judgeView.Rounds[0].Results) != 0 {
		t.Fatal("judge saw other judges' results")
	}
	for _, score := range judgeView.Rounds[0].Scores {
		if score.JudgeID != "j1" {
			t.Fatal("other judge scores exposed")
		}
	}
	if err := u.Transition(ctx, "event01", "round01", "close", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Submit(ctx, a, "round01", submission("after-close", "p01", 2, 1)); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("closed round accepted revision", err)
	}
	// A retry after a lost response remains idempotent, even after closure.
	if got, err := u.Submit(ctx, a, "round01", first); err != nil || got.Number != 2 {
		t.Fatal("successful operation could not be retried", err)
	}
	v, err := u.OperatorView(ctx, "event01")
	if err != nil {
		t.Fatal(err)
	}
	if v.Rounds[0].State != model.Closed || v.Rounds[0].Results[0].Average != 8 || v.Rounds[0].Results[1].Average != 7 {
		t.Fatal("incorrect equal-weight average", v.Rounds[0].Results)
	}
	event, err := r.Event(ctx, "event01")
	if err != nil || len(event.Revisions) != 49 {
		t.Fatal("revision history lost", err)
	}
	pending, err := r.Pending(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(pending)
	if strings.Contains(string(encoded), "password_hash") || strings.Contains(string(encoded), "$2a$") {
		t.Fatal("password hashes leaked into outbox")
	}
}
func TestValidationAndCancellation(t *testing.T) {
	_, u, a, _ := prepare(t)
	bad := submission("invalid-score", "p01", 0, 11)
	if _, err := u.Submit(ctx, a, "round01", bad); !errors.Is(err, apperror.ErrInvalid) {
		t.Fatal(err)
	}
	bad.Scores = map[string]int{"c1": 2}
	if _, err := u.Submit(ctx, a, "round01", bad); !errors.Is(err, apperror.ErrInvalid) {
		t.Fatal("missing criterion accepted", err)
	}
	if _, err := u.Submit(ctx, a, "round01", submission("other-participant", "missing", 0, 8)); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatal(err)
	}
	if err := u.Transition(ctx, "event01", "round01", "cancel", ""); !errors.Is(err, apperror.ErrInvalid) {
		t.Fatal("cancellation without reason accepted", err)
	}
	if err := u.Transition(ctx, "event01", "round01", "cancel", "Juri tidak dapat menyelesaikan penilaian"); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Submit(ctx, a, "round01", submission("cancelled-score", "p01", 0, 8)); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal(err)
	}
}
func TestConcurrentRetryHasOneRevisionAndOneOutboxEntry(t *testing.T) {
	r, u, a, _ := prepare(t)
	before, _ := r.PendingCount(ctx)
	sub := submission("same-operation", "p01", 0, 7)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := u.Submit(ctx, a, "round01", sub); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	e, _ := r.Event(ctx, "event01")
	after, _ := r.PendingCount(ctx)
	if len(e.Revisions) != 1 || after != before+1 {
		t.Fatal("retry duplicated persisted data")
	}
	sub.Scores["c1"] = 1
	if _, err := u.Submit(ctx, a, "round01", sub); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal("operation id reused for different payload", err)
	}
}
func TestOutboxFailureRollsBackAggregate(t *testing.T) {
	r, _, _, _ := prepare(t)
	pending, _ := r.Pending(ctx, 1)
	err := r.Update(ctx, "event01", func(e *model.Event) ([]exchange.Change, error) {
		e.Bundle.Name = "should roll back"
		return []exchange.Change{pending[0]}, nil
	})
	if err == nil {
		t.Fatal("duplicate outbox ID accepted")
	}
	e, err := r.Event(ctx, "event01")
	if err != nil || e.Bundle.Name == "should roll back" {
		t.Fatal("event committed without outbox", err)
	}
}
func TestSessionVersionAndLogout(t *testing.T) {
	r, u := open(t, filepath.Join(t.TempDir(), "arena.sqlite"))
	defer r.Close()
	b := bundle(t)
	if err := u.Prepare(ctx, b); err != nil {
		t.Fatal(err)
	}
	token, _, err := u.Login(ctx, b.ID, "juri1", "event-password", "client")
	if err != nil {
		t.Fatal(err)
	}
	b.Version = 2
	if err := u.Prepare(ctx, b); err != nil {
		t.Fatal(err)
	}
	if _, err := u.JudgeView(ctx, token); !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatal("old package session survived", err)
	}
	token, _, err = u.Login(ctx, b.ID, "juri1", "event-password", "client")
	if err != nil {
		t.Fatal(err)
	}
	if err := u.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := u.JudgeView(ctx, token); !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatal("logout did not revoke session", err)
	}
}
func TestCommitSurvivesProcessExitAndRecoveryInterruptsRound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arena.sqlite")
	r, u := open(t, path)
	if err := u.Prepare(ctx, bundle(t)); err != nil {
		t.Fatal(err)
	}
	if err := u.Transition(ctx, "event01", "round01", "start", ""); err != nil {
		t.Fatal(err)
	}
	token, _, err := u.Login(ctx, "event01", "juri1", "event-password", "client")
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	command := exec.Command(os.Args[0], "-test.run=^TestArenaCrashHelper$")
	command.Env = append(os.Environ(), "ARENA_CRASH_DATABASE="+path, "ARENA_CRASH_TOKEN="+token)
	if err := command.Run(); err == nil {
		t.Fatal("helper did not exit abruptly")
	} else {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 23 {
			t.Fatal("helper failed before the deliberate crash", err)
		}
	}
	r, u = open(t, path)
	defer r.Close()
	if err := u.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := u.JudgeView(ctx, token)
	if err != nil {
		t.Fatal("offline session not durable", err)
	}
	if v.Rounds[0].State != model.Interrupted || len(v.Rounds[0].Scores) != 1 {
		t.Fatal("committed score lost or judging resumed automatically")
	}
	if _, err := u.Submit(ctx, token, "round01", submission("after-restart", "p02", 0, 8)); !errors.Is(err, apperror.ErrConflict) {
		t.Fatal(err)
	}
	if err := u.Transition(ctx, "event01", "round01", "resume", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Submit(ctx, token, "round01", submission("after-restart", "p02", 0, 8)); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup", "arena.sqlite")
	if err := r.Backup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := repository.Open(backup)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	e, err := restored.Event(ctx, "event01")
	if err != nil || len(e.Revisions) != 2 {
		t.Fatal("backup did not restore committed revisions", err)
	}
}
func TestArenaCrashHelper(t *testing.T) {
	path := os.Getenv("ARENA_CRASH_DATABASE")
	if path == "" {
		return
	}
	r, err := repository.Open(path)
	if err != nil {
		os.Exit(20)
	}
	u := usecase.New(r, nil, usecase.Options{ArenaID: "arena01", EngineID: "engine01"})
	if _, err := u.Submit(ctx, os.Getenv("ARENA_CRASH_TOKEN"), "round01", submission("before-crash", "p01", 0, 9)); err != nil {
		os.Exit(21)
	}
	// No database Close/checkpoint: commits must already be durable in WAL.
	os.Exit(23)
}
