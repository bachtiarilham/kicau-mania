package usecase

import (
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
)

const Duration = 10 * time.Minute

type Participant struct {
	GantanganNumber int    `json:"gantanganNumber" binding:"required,min=1,max=24"`
	BirdName        string `json:"birdName" binding:"max=200"`
	OwnerName       string `json:"ownerName" binding:"max=200"`
}

type Round struct {
	ArenaID      string        `json:"arenaId" binding:"required,max=120"`
	EventID      string        `json:"eventId" binding:"required,max=120"`
	RoundID      string        `json:"roundId" binding:"required,max=120"`
	EventName    string        `json:"eventName" binding:"max=200"`
	SessionName  string        `json:"sessionName" binding:"max=200"`
	ArenaName    string        `json:"arenaName" binding:"max=200"`
	EventDate    string        `json:"eventDate" binding:"max=32"`
	RoundNumber  int           `json:"roundNumber"`
	StartTime    string        `json:"startTime" binding:"max=64"`
	EndTime      string        `json:"endTime" binding:"max=64"`
	Jurors       []string      `json:"jurors" binding:"max=4,dive,max=200"`
	JurorIDs     []string      `json:"jurorIds" binding:"max=4,dive,max=120"`
	Participants []Participant `json:"participants" binding:"max=24,dive"`
}

type Snapshot struct {
	Round
	State     string `json:"state"`
	ElapsedMS int64  `json:"elapsedMs"`
	RunID     string `json:"runId"`
}

type Rounds struct {
	mu        sync.Mutex
	now       func() time.Time
	snapshot  Snapshot
	startedAt time.Time
	completed map[[2]string]bool
}

func NewRounds(now func() time.Time) *Rounds {
	return &Rounds{now: now, completed: make(map[[2]string]bool)}
}

func (r *Rounds) Current() *Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.startedAt.IsZero() {
		return nil
	}
	snapshot := r.current()
	return &snapshot
}

func (r *Rounds) Completed(arenaID, roundID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completed[[2]string{arenaID, roundID}]
}

func (r *Rounds) Start(round Round) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.completed[[2]string{round.ArenaID, round.RoundID}] {
		return Snapshot{}, apperror.ErrConflict
	}
	if r.snapshot.State == "RUNNING" {
		if r.snapshot.ArenaID == round.ArenaID && r.snapshot.RoundID == round.RoundID {
			return r.current(), nil
		}
		return Snapshot{}, apperror.ErrConflict
	}
	r.startedAt = r.now()
	r.snapshot = Snapshot{Round: round, State: "RUNNING", RunID: r.startedAt.UTC().Format(time.RFC3339Nano)}
	return r.snapshot, nil
}

func (r *Rounds) Stop(arenaID, roundID string) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.snapshot.ArenaID != arenaID || r.snapshot.RoundID != roundID {
		return Snapshot{}, apperror.ErrNotFound
	}
	if r.snapshot.State != "RUNNING" {
		return r.snapshot, nil
	}
	r.snapshot = r.current()
	r.snapshot.State = "INCOMPLETE"
	if r.snapshot.ElapsedMS >= Duration.Milliseconds() {
		r.snapshot.State = "FINISHED"
		r.completed[[2]string{arenaID, roundID}] = true
	}
	return r.snapshot, nil
}

func (r *Rounds) current() Snapshot {
	snapshot := r.snapshot
	if snapshot.State == "RUNNING" {
		snapshot.ElapsedMS = min(max(0, r.now().Sub(r.startedAt)), Duration).Milliseconds()
	}
	return snapshot
}
