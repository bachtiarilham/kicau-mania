// Package model defines channel identities without upstream credentials.
package model

type Channel struct {
	ID string
}

// PlaybackChannel adds public playback information without exposing source credentials.
type PlaybackChannel struct {
	ID   string
	RTSP string
}
