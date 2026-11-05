package ffmpeg

import (
	"context"
	"io"

	"kicau.local/edge/internal/infra/runtime/subprocess"
)

type AudioDecoder struct{ Binary string }

func (f AudioDecoder) Audio(ctx context.Context, source string, write func([]byte)) error {
	r, w := io.Pipe()
	defer r.Close()
	args := append(input(source), "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
	p, e := subprocess.Start(ctx, f.Binary, args, w)
	if e != nil {
		w.Close()
		return e
	}
	go func() { <-p.Done(); w.CloseWithError(p.Err()) }()
	for {
		b := make([]byte, 6400)
		_, e = io.ReadFull(r, b)
		if e != nil {
			break
		}
		write(b)
	}
	p.Stop()
	return p.Err()
}
