package model

import (
	"context"
	"path"
	"regexp"
	"strings"
	"time"
)

const PartSize int64 = 16 * 1024 * 1024

type Mapping struct {
	ChannelID      string `json:"channel_id"`
	RegistrationID string `json:"registration_id"`
	GantanganID    string `json:"gantangan_id"`
}
type Ingestion struct {
	EngineID  string    `json:"engine_id"`
	SessionID string    `json:"session_id"`
	RoundID   string    `json:"round_id"`
	Mappings  []Mapping `json:"mappings"`
}
type Upload struct {
	DisplayFilename string    `json:"display_filename"`
	Container       string    `json:"container"`
	SessionID       string    `json:"session_id"`
	ChannelID       string    `json:"channel_id"`
	RecordingID     string    `json:"recording_id"`
	Size            int64     `json:"size_bytes"`
	SHA256          string    `json:"sha256"`
	Started         time.Time `json:"started_at"`
	Ended           time.Time `json:"ended_at"`
}
type Part struct {
	Number int32  `json:"number"`
	ETag   string `json:"etag"`
	SHA256 string `json:"sha256"`
}
type SignPart struct {
	Number int32  `json:"number"`
	SHA256 string `json:"sha256"`
}
type Signed struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}
type Objects interface {
	Begin(context.Context, string, string) (string, error)
	Sign(context.Context, string, string, SignPart) (Signed, error)
	Complete(context.Context, string, string, []Part) error
	Exists(context.Context, string, int64, string) (bool, error)
	Parts(context.Context, string, string) ([]Part, error)
	Download(context.Context, string) (string, error)
}

type WhatsAppDeliveryResult struct {
	RecordingID    string `json:"recording_id"`
	RegistrationID string `json:"registration_id"`
	RecipientPhone string `json:"recipient_phone"`
	Status         string `json:"status"` // SENT, FAILED, SKIPPED
	StreamingURL   string `json:"streaming_url"`
	Error          string `json:"error,omitempty"`
}

type PublicVideoWatch struct {
	RecordingID     string `json:"recording_id"`
	BirdName        string `json:"bird_name"`
	OwnerName       string `json:"owner_name"`
	GantanganCode   string `json:"gantangan_code"`
	StreamingURL    string `json:"streaming_url"`
	DownloadURL     string `json:"download_url"`
	Duration        string `json:"duration"`
	DisplayFilename string `json:"display_filename,omitempty"`
	VideoNote       string `json:"video_note,omitempty"`
}

const LimitedRecordingNote = "Rekaman ini mungkin tidak lengkap atau tidak dapat diputar dengan lancar."

var recordingNames = regexp.MustCompile(`^Gantangan (0[1-9]|1[0-9]|2[0-4])( - Rekaman Terbatas)?\.mkv$`)

func ValidRecordingName(name string) bool { return recordingNames.MatchString(name) }

func RecordingName(key string) string {
	name := path.Base(key)
	if !ValidRecordingName(name) {
		return ""
	}
	return name
}

func LimitedRecording(key string) bool {
	return strings.HasSuffix(RecordingName(key), " - Rekaman Terbatas.mkv")
}
