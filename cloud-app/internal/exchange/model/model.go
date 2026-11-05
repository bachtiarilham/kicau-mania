package model

import (
	"encoding/json"
	"errors"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized")
var ErrInvalid = errors.New("invalid request")
var ErrConflict = errors.New("identity reused with different content")
var ErrNotFound = errors.New("not found")

type Package struct {
	ID       string `json:"id"`
	ArenaID  string `json:"arena_id"`
	EngineID string `json:"engine_id"`
	Version  int    `json:"version"`
}

type Engine struct {
	ArenaID  string `json:"arena_id"`
	EngineID string `json:"engine_id"`
	Token    string `json:"token"`
}
type Change struct {
	ID       string          `json:"id"`
	ArenaID  string          `json:"arena_id"`
	EngineID string          `json:"engine_id"`
	EventID  string          `json:"event_id"`
	Kind     string          `json:"kind"`
	At       time.Time       `json:"at"`
	Payload  json.RawMessage `json:"payload"`
}
type Command struct {
	ID        string          `json:"id"`
	ArenaID   string          `json:"arena_id"`
	EngineID  string          `json:"engine_id"`
	Kind      string          `json:"kind"`
	Payload   json.RawMessage `json:"payload"`
	ExpiresAt time.Time       `json:"expires_at"`
}
type Result struct {
	ID    string    `json:"id"`
	State string    `json:"state"`
	At    time.Time `json:"at"`
}
type CommandState struct {
	Command Command `json:"command"`
	Result  *Result `json:"result,omitempty"`
	State   string  `json:"state"`
}
