// Recording metadata describes persisted media state sent to cloud.
package model

import (
	"time"
)

type MediaUpdate struct {
	ID          string    `json:"id"`
	ArenaID     string    `json:"arena_id"`
	EngineID    string    `json:"engine_id"`
	RecordingID string    `json:"recording_id"`
	SessionID   string    `json:"session_id"`
	ChannelID   string    `json:"channel_id"`
	Version     int64     `json:"version"`
	State       string    `json:"state"`
	ObjectKey   string    `json:"object_key"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
}
