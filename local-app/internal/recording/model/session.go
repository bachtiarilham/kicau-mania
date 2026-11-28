package model

import "time"

type Restream struct {
	ChannelID string `json:"channel_id"`
	Target    string `json:"target"`
}

type StartRequest struct {
	ID          string       `json:"id" binding:"required"`
	Channels    []string     `json:"channels" binding:"required,min=1,max=24"`
	Restreams   []Restream   `json:"restreams"`
	Competition *Competition `json:"competition,omitempty"`
}

type Competition struct {
	ArenaID     string         `json:"arena_id"`
	ArenaName   string         `json:"arena_name"`
	EventID     string         `json:"event_id"`
	EventName   string         `json:"event_name"`
	EventDate   string         `json:"event_date"`
	RoundID     string         `json:"round_id"`
	RoundNumber int            `json:"round_number"`
	Gantangan   map[string]int `json:"gantangan"`
}

type Channel struct {
	ErrorCode      string `json:"error_code,omitempty"`
	RecordingState string `json:"recording_state,omitempty"`
	SegmentCount   int    `json:"segment_count"`
	Gaps           []Gap  `json:"gaps,omitempty"`
	ID             string `json:"id"`
	RecordingID    string `json:"recording_id"`
	State          string `json:"state"`
	Attempts       int    `json:"attempts"`
	Error          string `json:"error,omitempty"`
}

type Session struct {
	StopReason string `json:"stop_reason,omitempty"`
	StartRequest
	State           string    `json:"state"`
	StartedAt       time.Time `json:"started_at"`
	StoppedAt       time.Time `json:"stopped_at,omitempty"`
	ChannelsState   []Channel `json:"channel_states"`
	Error           string    `json:"error,omitempty"`
	RecordingRoot   string    `json:"recording_root,omitempty"`
	RecordingFolder string    `json:"recording_folder,omitempty"`
	RoundComplete   bool      `json:"round_complete,omitempty"`
}

func Terminal(state string) bool {
	return state == "STOPPED" || state == "INTERRUPTED" || state == "FAILED"
}
