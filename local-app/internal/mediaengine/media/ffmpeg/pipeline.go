package ffmpeg

import (
	"context"
	"io"
	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/mediaengine/media"
	"strconv"
	"strings"
	"sync"
)

// Pipeline records A/V and decodes PCM from the same RTSP input.
type Pipeline struct {
	Binary   string
	Profiles map[string]media.StreamProfile
}

func (f Pipeline) Profile(source string) media.StreamProfile {
	if v, ok := f.Profiles[source]; ok {
		return v
	}
	return media.StreamProfile{HasAudio: true}
}

// progress never forwards stderr text, which can contain source credentials.
type progress struct {
	code, message string
	pending       string
	frame         int
	once          sync.Once
	ready         func()
}

func (p *progress) Write(b []byte) (int, error) {
	n := len(b)
	p.pending += string(b)
	for {
		line, rest, ok := strings.Cut(p.pending, "\n")
		if !ok {
			break
		}
		p.pending = rest
		text := strings.ToLower(line)
		switch {
		case strings.Contains(text, "401 unauthorized") || strings.Contains(text, "403 forbidden"):
			p.code, p.message = "DEVICE_AUTH_FAILED", "Kredensial sumber ditolak"
		case strings.Contains(text, "no space left"):
			p.code, p.message = "DISK_FULL", "Ruang penyimpanan habis"
		case strings.Contains(text, "unknown encoder"):
			p.code, p.message = "ENCODER_MISSING", "Encoder media tidak tersedia"
		case strings.Contains(text, "connection refused") || strings.Contains(text, "connection timed out"):
			p.code, p.message = "SOURCE_UNREACHABLE", "Koneksi media tidak tersedia"
		}
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		if key == "frame" {
			p.frame, _ = strconv.Atoi(strings.TrimSpace(value))
		}
		if key == "out_time_us" && p.frame > 0 {
			v, _ := strconv.ParseInt(value, 10, 64)
			if v > 0 {
				p.once.Do(p.ready)
			}
		}
	}
	if len(p.pending) > 8192 {
		p.pending = ""
	}
	return n, nil
}

func (f Pipeline) RunMedia(ctx context.Context, source, file string, ready func(), consume func([]byte)) error {
	profile := f.Profile(source)
	inputURL := source
	if profile.InputURL != "" {
		inputURL = profile.InputURL
	}
	r, w := io.Pipe()
	defer r.Close()
	args := append([]string{"-progress", "pipe:2", "-nostats", "-stats_period", "0.5"}, input(inputURL)...)
	args = append(args, "-map", "0:v:0", "-c:v", "copy")
	if profile.HasAudio {
		args = append(args, "-map", "0:a:0", "-c:a", "aac")
	}
	args = append(args, "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-flush_packets", "1", "-frag_duration", "1000000", "-f", "mp4", "-n", file)
	if profile.HasAudio {
		args = append(args, "-map", "0:a:0", "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
	}
	if profile.PublishURL != "" {
		args = append(args, "-map", "0:v:0")
		if profile.Transcode {
			args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-tune", "zerolatency", "-profile:v", "baseline", "-pix_fmt", "yuv420p", "-vf", "scale=640:360:force_original_aspect_ratio=decrease,pad=640:360:(ow-iw)/2:(oh-ih)/2", "-r", "15", "-g", "30", "-bf", "0", "-b:v", "750k")
		} else {
			args = append(args, "-c:v", "copy")
		}
		if profile.HasAudio {
			args = append(args, "-map", "0:a:0", "-c:a", "libopus", "-ar", "48000", "-ac", "1", "-b:a", "64k")
		}
		args = append(args, "-f", "rtsp", "-rtsp_transport", "tcp", profile.PublishURL)
	}
	// Session cancellation is handled with Stop (q + bounded kill), not an
	// immediate CommandContext kill that would skip recording finalization.
	diagnostics := &progress{ready: ready}
	p, err := subprocess.StartWithDiagnostics(context.WithoutCancel(ctx), f.Binary, args, w, diagnostics)
	if err != nil {
		w.Close()
		return &media.PipelineError{Code: "BINARY_START_FAILED", Message: "Proses FFmpeg tidak dapat dijalankan"}
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
	if err := p.Err(); err != nil && ctx.Err() == nil {
		code, message := diagnostics.code, diagnostics.message
		if code == "" {
			code, message = "SOURCE_INTERRUPTED", "Pipeline media terputus; mencoba ulang"
		}
		return &media.PipelineError{Code: code, Message: message}
	}
	return p.Err()
}
