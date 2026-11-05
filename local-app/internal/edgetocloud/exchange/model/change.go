// Package model defines transport-neutral changes exchanged with the cloud.
package model

import (
	"encoding/json"
	"time"
)

type Change struct {
	ID       string          `json:"id"`
	ArenaID  string          `json:"arena_id"`
	EngineID string          `json:"engine_id"`
	EventID  string          `json:"event_id"`
	Kind     string          `json:"kind"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload"`
}
