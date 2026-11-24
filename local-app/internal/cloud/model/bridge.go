package model

import "time"

type Pairing struct {
	Base     string `json:"base_url"`
	Token    string `json:"token,omitempty"`
	EngineID string `json:"engine_id"`
	ArenaID  string `json:"arena_id"`
}

type Mapping struct {
	ChannelID      string `json:"channel_id"`
	RegistrationID string `json:"registration_id"`
	GantanganID    string `json:"gantangan_id"`
}

type Ingestion struct {
	ID        string    `json:"id"`
	EngineID  string    `json:"engine_id"`
	SessionID string    `json:"session_id"`
	RoundID   string    `json:"round_id"`
	Mappings  []Mapping `json:"mappings"`
}

type Progress struct {
	SHA256   string    `json:"sha256,omitempty"`
	Modified time.Time `json:"file_modified_at,omitempty"`
	State    string    `json:"upload_state"`
	Bytes    int64     `json:"uploaded_bytes"`
	Size     int64     `json:"size_bytes"`
	UploadID string    `json:"upload_id,omitempty"`
	Error    string    `json:"upload_error,omitempty"`
	Updated  time.Time `json:"upload_updated_at"`
}
