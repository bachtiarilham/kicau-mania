package model

import "time"

type Metric struct {
	ChannelID string    `json:"channel_id"`
	At        time.Time `json:"at"`
	PeakDBFS  float64   `json:"peak_dbfs"`
	RMSDBFS   float64   `json:"rms_dbfs"`
	ActiveMS  int64     `json:"active_ms"`
	WindowMS  int64     `json:"window_ms"`
}
