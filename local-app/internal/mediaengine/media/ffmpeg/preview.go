package ffmpeg

import (
	"context"
	"io"
	"kicau.local/edge/internal/infra/runtime/subprocess"
)

// RunPreviewAudio reads levels without decoding video or opening a recording.
func (f Pipeline) RunPreviewAudio(ctx context.Context, source string, consume func([]byte)) error {
	args := append([]string{"-allowed_media_types", "audio"}, input(source)...)
	args = append(args, "-map", "0:a:0", "-vn")
	if filter := f.AudioFilterString(); filter != "" {
		args = append(args, "-af", filter)
	}
	args = append(args, "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	p, err := subprocess.Start(ctx, f.Binary, args, w)
	if err != nil {
		return err
	}
	go func() {
		<-p.Done()
		w.Close()
	}()
	frame := make([]byte, 6400)
	for {
		if _, err := io.ReadFull(r, frame); err != nil {
			break
		}
		consume(frame)
	}
	return p.Err()
}
