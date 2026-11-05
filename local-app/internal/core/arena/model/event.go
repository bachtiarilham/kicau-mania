// Package model defines the event snapshot, scoring history, and offline state.
package model

import (
	"time"
)

const (
	Prepared    = "PREPARED"
	Active      = "ACTIVE"
	Interrupted = "INTERRUPTED"
	Closed      = "CLOSED"
	Cancelled   = "CANCELLED"
)

// Bundle is prepared before the event. It contains event-specific password
// hashes, never cloud account credentials. It is accepted only by the operator API.
type Bundle struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Version  int         `json:"version"`
	ArenaID  string      `json:"arena_id"`
	EngineID string      `json:"engine_id"`
	Judges   []Judge     `json:"judges"`
	Criteria []Criterion `json:"criteria"`
	Rounds   []Round     `json:"rounds"`
}
type Judge struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash,omitempty"`
}
type Criterion struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Participant struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Lapak     int    `json:"lapak"`
	ChannelID string `json:"channel_id"`
}
type Round struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Participants []Participant `json:"participants"`
	JudgeIDs     []string      `json:"judge_ids"`
	State        string        `json:"state"`
	Reason       string        `json:"reason,omitempty"`
}
type Event struct {
	Bundle      Bundle     `json:"bundle"`
	Fingerprint string     `json:"fingerprint"`
	Revisions   []Revision `json:"revisions"`
	Audit       []Audit    `json:"audit"`
}
type Submission struct {
	EventID          string         `json:"event_id,omitempty"`
	JudgeID          string         `json:"judge_id,omitempty"`
	OperationID      string         `json:"operation_id"`
	ParticipantID    string         `json:"participant_id"`
	ExpectedRevision int            `json:"expected_revision"`
	Scores           map[string]int `json:"scores"`
}
type Revision struct {
	OperationID string `json:"operation_id"`
	// The hash must survive restart but is not exposed to the judge client.
	Fingerprint   string         `json:"fingerprint,omitempty"`
	RoundID       string         `json:"round_id"`
	JudgeID       string         `json:"judge_id"`
	ParticipantID string         `json:"participant_id"`
	Number        int            `json:"revision"`
	Scores        map[string]int `json:"scores"`
	At            time.Time      `json:"at"`
}
type Audit struct {
	ID      string    `json:"id"`
	RoundID string    `json:"round_id,omitempty"`
	Actor   string    `json:"actor"`
	Action  string    `json:"action"`
	Reason  string    `json:"reason,omitempty"`
	At      time.Time `json:"at"`
}
type Session struct {
	TokenHash string
	EventID   string
	JudgeID   string
	Version   int
	ExpiresAt time.Time
}
type ScoreResult struct {
	ParticipantID string  `json:"participant_id"`
	Count         int     `json:"count"`
	Required      int     `json:"required"`
	Average       float64 `json:"average"`
}

// Latest returns the accepted revision per judge/participant for a round.
func Latest(e Event, roundID string) map[string]Revision {
	out := map[string]Revision{}
	for _, r := range e.Revisions {
		if r.RoundID == roundID {
			out[r.JudgeID+":"+r.ParticipantID] = r
		}
	}
	return out
}
func Results(e Event, round Round) []ScoreResult {
	latest := Latest(e, round.ID)
	out := make([]ScoreResult, 0, len(round.Participants))
	for _, p := range round.Participants {
		result := ScoreResult{ParticipantID: p.ID, Required: len(round.JudgeIDs) * len(e.Bundle.Criteria)}
		sum := 0
		for _, judge := range round.JudgeIDs {
			for _, score := range latest[judge+":"+p.ID].Scores {
				result.Count++
				sum += score
			}
		}
		if result.Count > 0 {
			result.Average = float64(sum) / float64(result.Count)
		}
		out = append(out, result)
	}
	return out
}
