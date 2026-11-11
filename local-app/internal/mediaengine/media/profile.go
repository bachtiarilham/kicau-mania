package media

// StreamProfile describes session output policy without exposing source credentials.
type StreamProfile struct {
	InputURL   string
	HasAudio   bool
	Transcode  bool
	PublishURL string
}
