package ffmpeg

import (
	"context"

	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/infra/media"
)

type Restreamer struct{ Binary, Probe string }

func (f Restreamer) Restream(ctx context.Context, source, target string) (media.Process, error) {
	if _, err := (Inspector{ProbeBinary: f.Probe}).Probe(ctx, source); err != nil {
		return nil, err
	}
	args := append(input(source), "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy")
	if len(target) >= 4 && target[:4] == "rtsp" {
		args = append(args, "-f", "rtsp", "-rtsp_transport", "tcp", target)
	} else {
		args = append(args, "-f", "flv", target)
	}
	return subprocess.Start(ctx, f.Binary, args, nil)
}
