package ffmpeg

import (
	"context"

	"kicau.local/edge/internal/media"
	"kicau.local/edge/internal/runtime/subprocess"
)

type Recorder struct{ Binary string }

func (f Recorder) Record(ctx context.Context, source, file string) (media.Process, error) {
	args := append(input(source), "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "copy", "-c:a", "aac", "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-f", "mp4", "-n", file)
	return subprocess.Start(ctx, f.Binary, args, nil)
}
