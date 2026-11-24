package model

import "time"

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (f *Failure) Error() string { return f.Message }

type Probe struct {
	VideoCodec string    `json:"video_codec"`
	AudioCodec string    `json:"audio_codec,omitempty"`
	HasAudio   bool      `json:"has_audio"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	FPS        float64   `json:"fps"`
	BFrames    int       `json:"b_frames"`
	TestedAt   time.Time `json:"tested_at"`
}

type Diagnostic struct {
	Component string `json:"component"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

func ValidateMedia(p Probe) error {
	if p.VideoCodec != "hevc" {
		return &Failure{Code: "UNSUPPORTED_VIDEO_CODEC", Message: "NVR harus menggunakan video HEVC/H.265; konversi codec tidak tersedia"}
	}
	return nil
}
