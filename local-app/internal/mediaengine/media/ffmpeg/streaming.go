package ffmpeg

import (
	"context"

	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/mediaengine/media"
)

type Restreamer struct{ Binary string }

func (f Restreamer) Restream(ctx context.Context, source, target string) (media.Process, error) {
	args := append(input(source), "-map", "0:v:0", "-map", "0:a:0?", "-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-pix_fmt", "yuv420p", "-r", "25", "-g", "50", "-b:v", "3000k", "-maxrate", "3000k", "-bufsize", "6000k", "-c:a", "aac", "-b:a", "128k", "-ar", "44100")
	if len(target) >= 4 && target[:4] == "rtsp" {
		args = append(args, "-f", "rtsp", "-rtsp_transport", "tcp", target)
	} else {
		args = append(args, "-f", "flv", target)
	}
	return subprocess.Start(ctx, f.Binary, args, nil)
}
