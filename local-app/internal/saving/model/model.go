package model

import "time"

type StartRound struct {
	ID       string   `json:"id" binding:"required"`
	Channels []string `json:"channels" binding:"required,min=1,max=24"`
}

type Recording struct {
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
