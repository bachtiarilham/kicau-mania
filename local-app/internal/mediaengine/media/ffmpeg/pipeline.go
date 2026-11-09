package ffmpeg

import (
	"context"
	"io"
	"kicau.local/edge/internal/infra/runtime/subprocess"
	"sync"
)

// Pipeline records A/V and decodes PCM from the same RTSP input.
type Pipeline struct{ Binary string }

func (f Pipeline) Run(ctx context.Context, source, file string, consume func([]byte)) error {
	r, w := io.Pipe()
	defer r.Close()
	args := append(input(source), "-map", "0:v:0", "-map", "0:a:0", "-c:v", "copy", "-c:a", "aac", "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-flush_packets", "1", "-frag_duration", "1000000", "-f", "mp4", "-n", file,
		"-map", "0:a:0", "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
	// Session cancellation is handled with Stop (q + bounded kill), not an
	// immediate CommandContext kill that would skip recording finalization.
	p, err := subprocess.Start(context.WithoutCancel(ctx), f.Binary, args, w)
	if err != nil {
		w.Close()
		return err
	}
	frames := make(chan []byte, 16)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(frames)
		for {
			b := make([]byte, 6400)
			if _, err := io.ReadFull(r, b); err != nil {
				return
			}
			select {
			case frames <- b:
			default:
			}
		}
	}()
	go func() {
		defer wg.Done()
		for b := range frames {
			if ctx.Err() == nil {
				consume(b)
			}
		}
	}()
	select {
	case <-ctx.Done():
		p.Stop()
	case <-p.Done():
	}
	w.Close()
	wg.Wait()
	return p.Err()
}
