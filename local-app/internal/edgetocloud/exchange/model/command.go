package model

import (
	"encoding/json"
	"time"
)

type Command struct {
	ID        string          `json:"id"`
	ArenaID   string          `json:"arena_id"`
	EngineID  string          `json:"engine_id"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt time.Time       `json:"expires_at"`
}
type CommandResult struct {
	ID    string    `json:"id"`
	State string    `json:"state"`
	At    time.Time `json:"at"`
}
