// Heartbeat reports operational state without camera or storage credentials.
package model

import (
	"time"
)

type Heartbeat struct {
	ArenaID        string    `json:"arena_id"`
	EngineID       string    `json:"engine_id"`
	At             time.Time `json:"at"`
	MediaState     string    `json:"media_state"`
	Channels       []string  `json:"channels"`
	RecordingCount int       `json:"recording_count"`
	PendingUploads int       `json:"pending_uploads"`
	RestreamCount  int       `json:"restream_count"`
}
