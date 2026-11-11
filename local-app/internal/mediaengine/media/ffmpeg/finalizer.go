package ffmpeg

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"kicau.local/edge/internal/mediaengine/media"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Finalizer validates packet-bearing A/V streams and joins only compatible parts.
// Inputs and recovery artifacts are retained; final output is committed by rename.
type Finalizer struct{ Binary, Probe string }
type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 2*1024*1024 {
		return 0, media.FinalizationError("probe output too large")
	}
	return b.Buffer.Write(p)
}
func (f Finalizer) signature(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, f.Probe, "-v", "error", "-count_packets", "-show_data", "-show_entries", "stream=codec_type,codec_name,width,height,pix_fmt,profile,level,sample_rate,channels,channel_layout,extradata,nb_read_packets:format=duration", "-of", "json", path)
	var out boundedOutput
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return "", media.FinalizationError("recording probe failed")
	}
	var data struct {
		Streams []map[string]any `json:"streams"`
		Format  struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if json.Unmarshal(out.Bytes(), &data) != nil {
		return "", media.FinalizationError("invalid recording probe")
	}
	d, err := strconv.ParseFloat(data.Format.Duration, 64)
	if err != nil || d <= 0 || math.IsNaN(d) || math.IsInf(d, 0) {
		return "", media.FinalizationError("recording has no valid duration")
	}
	streams := map[string]map[string]any{}
	for _, s := range data.Streams {
		k, _ := s["codec_type"].(string)
		if k != "video" && k != "audio" {
			continue
		}
		n, _ := strconv.Atoi(fmtString(s["nb_read_packets"]))
		if n < 1 || streams[k] != nil {
			return "", media.FinalizationError("recording has no complete A/V packets")
		}
		delete(s, "nb_read_packets")
		streams[k] = s
	}
	if streams["video"] == nil {
		return "", media.FinalizationError("recording requires video")
	}
	b, _ := json.Marshal(streams)
	return string(b), nil
}
func fmtString(v any) string { s, _ := v.(string); return s }
func (f Finalizer) remux(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, f.Binary, append([]string{"-hide_banner", "-loglevel", "error", "-nostdin"}, args...)...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		return media.FinalizationError("recording remux failed")
	}
	return nil
}
func (f Finalizer) Finalize(ctx context.Context, parts []string, output string) error {
	if _, err := os.Stat(output); err == nil {
		_, err = f.signature(ctx, output)
		return err
	} else if !os.IsNotExist(err) {
		return media.FinalizationError("recording output unavailable")
	}
	if len(parts) == 0 {
		return media.FinalizationError("no recording segments")
	}
	var signature string
	valid := []string{}
	for _, part := range parts {
		// Never trust byte count alone. Remux partials into separate artifacts,
		// preserving the original even when FFmpeg exits on a truncated tail.
		file := part
		if strings.HasSuffix(part, ".partial") {
			file = part + ".salvage.mp4"
			if _, err := f.signature(ctx, file); err != nil {
				tmp, err := os.CreateTemp(filepath.Dir(output), ".salvage-*.mp4")
				if err != nil {
					return media.FinalizationError("cannot prepare salvage")
				}
				name := tmp.Name()
				tmp.Close()
				os.Remove(name)
				_ = f.remux(ctx, "-i", part, "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-f", "mp4", "-n", name)
				if _, err := f.signature(ctx, name); err != nil {
					os.Remove(name)
					return media.FinalizationError("recording segment cannot be recovered")
				}
				if err := os.Rename(name, file); err != nil {
					os.Remove(name)
					return media.FinalizationError("cannot commit recovered segment")
				}
			}
		}
		sig, err := f.signature(ctx, file)
		if err != nil {
			return err
		}
		if signature != "" && sig != signature {
			return media.FinalizationError("recording segment codecs changed")
		}
		signature = sig
		valid = append(valid, file)
	}
	list, err := os.CreateTemp(filepath.Dir(output), ".concat-*.txt")
	if err != nil {
		return media.FinalizationError("cannot prepare recording list")
	}
	defer os.Remove(list.Name())
	for _, file := range valid {
		absolute, err := filepath.Abs(file)
		if err != nil {
			list.Close()
			return media.FinalizationError("invalid recording path")
		}
		quoted := strings.ReplaceAll(filepath.ToSlash(absolute), "'", "'\\''")
		if strings.ContainsAny(quoted, "\r\n") {
			list.Close()
			return media.FinalizationError("invalid recording path")
		}
		if _, err := io.WriteString(list, "file '"+quoted+"'\n"); err != nil {
			list.Close()
			return media.FinalizationError("cannot write recording list")
		}
	}
	if list.Close() != nil {
		return media.FinalizationError("cannot close recording list")
	}
	tmp, err := os.CreateTemp(filepath.Dir(output), ".final-*.mp4")
	if err != nil {
		return media.FinalizationError("cannot prepare final recording")
	}
	name := tmp.Name()
	tmp.Close()
	os.Remove(name)
	defer os.Remove(name)
	if err := f.remux(ctx, "-f", "concat", "-safe", "0", "-i", list.Name(), "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy", "-movflags", "+frag_keyframe+empty_moov+default_base_moof", "-f", "mp4", "-n", name); err != nil {
		return err
	}
	if _, err := f.signature(ctx, name); err != nil {
		return err
	}
	file, err := os.OpenFile(name, os.O_RDWR, 0600)
	if err != nil {
		return media.FinalizationError("cannot flush final recording")
	}
	err = file.Sync()
	file.Close()
	if err != nil {
		return media.FinalizationError("cannot flush final recording")
	}
	if os.Rename(name, output) != nil {
		return media.FinalizationError("cannot commit final recording")
	}
	return nil
}
