// Playback messages carry scoped authorization requests and grants.
package model

import (
	"time"
)

type PlaybackRequest struct {
	Ticket    string `json:"ticket"`
	ArenaID   string `json:"arena_id"`
	EngineID  string `json:"engine_id"`
	ChannelID string `json:"channel_id"`
}
type PlaybackGrant struct {
	Allowed   bool      `json:"allowed"`
	UserID    string    `json:"user_id"`
	ArenaID   string    `json:"arena_id"`
	EngineID  string    `json:"engine_id"`
	ChannelID string    `json:"channel_id"`
	ExpiresAt time.Time `json:"expires_at"`
}
