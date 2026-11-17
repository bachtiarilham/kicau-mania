package model

import (
	"errors"
	"time"
)

var ErrNotMapped = errors.New("recording has no approved cloud mapping")

type Recording struct {
	Segments    []Segment `json:"segments,omitempty"`
	Gaps        []Gap     `json:"gaps,omitempty"`
	Error       string    `json:"error,omitempty"`
	Version     int64     `json:"version,omitempty"`
	ID          string    `json:"id"`
	RoundID     string    `json:"round_id"`
	ChannelID   string    `json:"channel_id"`
	State       string    `json:"state"`
	File        string    `json:"file"`
	Key         string    `json:"object_key"`
	Attempts    int       `json:"attempts"`
	NextAttempt time.Time `json:"next_attempt"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
}

type Segment struct {
	File      string    `json:"file"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	HasMedia  bool      `json:"has_media"`
}
type Gap struct {
	Reason    string    `json:"reason,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
