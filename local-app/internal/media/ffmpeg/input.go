package ffmpeg

func input(source string) []string {
	return []string{"-hide_banner", "-loglevel", "error", "-rtsp_transport", "tcp", "-timeout", "10000000", "-i", source}
}
