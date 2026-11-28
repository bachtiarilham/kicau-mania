package usecase

import (
	"errors"
	"testing"
	"time"

	"kicau.local/edge/internal/apperror"
)

func TestRoundClockAndTransitions(t *testing.T) {
	now := time.Now()
	rounds := NewRounds(func() time.Time { return now })
	if rounds.Current() != nil {
		t.Fatal("unexpected round")
	}
	round := Round{ArenaID: "arena", RoundID: "round"}
	if state, err := rounds.Start(round); err != nil || state.State != "RUNNING" || state.ElapsedMS != 0 {
		t.Fatalf("start: %+v, %v", state, err)
	}
	now = now.Add(2*time.Minute + 300*time.Millisecond)
	if current := rounds.Current(); current.ElapsedMS != 120300 {
		t.Fatalf("current: %+v", current)
	}
	if again, err := rounds.Start(round); err != nil || again.ElapsedMS != 120300 {
		t.Fatalf("start retry reset the clock: %+v, %v", again, err)
	}
	if _, err := rounds.Start(Round{ArenaID: "arena", RoundID: "other"}); !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("start replaced active round: %v", err)
	}
	if _, err := rounds.Stop("other", round.RoundID); !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("stop of wrong arena: %v", err)
	}
	if state, err := rounds.Stop(round.ArenaID, round.RoundID); err != nil || state.State != "FINISHED" || state.ElapsedMS != 120300 {
		t.Fatalf("stop: %+v, %v", state, err)
	}
	now = now.Add(time.Minute)
	if state := rounds.Current(); state.ElapsedMS != 120300 {
		t.Fatalf("finished clock progressed: %+v", state)
	}
	if _, err := rounds.Start(round); err != nil {
		t.Fatal(err)
	}
	now = now.Add(11 * time.Minute)
	if state := rounds.Current(); state.State != "RUNNING" || state.ElapsedMS != 600000 {
		t.Fatalf("time up must wait for stop: %+v", state)
	}
}
