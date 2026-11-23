package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/mediaengine/media"
	"strconv"
	"strings"
	"sync"
)

type AudioFilterOptions struct {
	Enabled        bool
	HighpassFreq   int
	GateThreshold  float64
	GateRatio      float64
	GateAttack     float64
	GateRelease    float64
	ApplyRecording bool
}

// Pipeline records A/V and decodes PCM from the same RTSP input.
type Pipeline struct {
	Binary, Probe string
	AudioFilter   AudioFilterOptions
}

func (f Pipeline) AudioFilterString() string {
	if !f.AudioFilter.Enabled {
		return ""
	}
	freq := f.AudioFilter.HighpassFreq
	if freq <= 0 {
		freq = 100
	}
	ratio := f.AudioFilter.GateRatio
	if ratio <= 0 {
		ratio = 4.0
	}
	attack := f.AudioFilter.GateAttack
	if attack <= 0 {
		attack = 5.0
	}
	release := f.AudioFilter.GateRelease
	if release <= 0 {
		release = 150.0
	}
	thresh := f.AudioFilter.GateThreshold
	if thresh == 0 {
		thresh = -55.0
	}
	return fmt.Sprintf("highpass=f=%d,agate=threshold=%.1fdB:ratio=%.1f:attack=%.1f:release=%.1f",
		freq, thresh, ratio, attack, release)
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
		case strings.Contains(text, "could not find tag for codec") || strings.Contains(text, "codec not supported"):
			p.code, p.message = "UNSUPPORTED_MEDIA_CODEC", "Codec audio sumber tidak dapat disimpan dalam MKV tanpa konversi"
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
	observed, err := (Inspector{ProbeBinary: f.Probe}).Probe(ctx, source)
	if err != nil {
		return &media.PipelineError{Code: "SOURCE_VALIDATION_FAILED", Message: "Sumber harus HEVC/H.265 dan dapat dibaca; periksa probe NVR"}
	}
	r, w := io.Pipe()
	defer r.Close()
	args := append([]string{"-progress", "pipe:2", "-nostats", "-stats_period", "0.5"}, input(source)...)
	args = append(args, "-map", "0:v:0", "-c:v", "copy")
	if observed.HasAudio {
		if f.AudioFilter.ApplyRecording && f.AudioFilterString() != "" {
			args = append(args, "-map", "0:a:0", "-af", f.AudioFilterString(), "-c:a", "aac", "-b:a", "128k")
		} else {
			args = append(args, "-map", "0:a:0", "-c:a", "copy")
		}
	}
	// Fill only missing presentation timestamps from decode timestamps.
	// This changes container timing metadata, never encoded video packets.
	args = append(args, "-bsf:v", "setts=pts=if(eq(PTS\\,NOPTS)\\,DTS\\,PTS)")
	args = append(args, "-flush_packets", "1", "-cluster_time_limit", "1000", "-f", "matroska", "-n", file)
	if observed.HasAudio {
		pcmArgs := []string{"-map", "0:a:0", "-vn"}
		if filter := f.AudioFilterString(); filter != "" {
			pcmArgs = append(pcmArgs, "-af", filter)
		}
		pcmArgs = append(pcmArgs, "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1")
		args = append(args, pcmArgs...)
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
