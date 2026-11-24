package ffmpeg

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"kicau.local/edge/internal/camera/model"
)

// Inspector reads a bounded sample; a successful TCP connection alone is not a valid stream.
type Inspector struct{ Binary, ProbeBinary, MediaBinary string }

func (f Inspector) Probe(ctx context.Context, source string) (model.Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var out, diagnostics boundedOutput
	cmd := exec.CommandContext(ctx, f.ProbeBinary, "-v", "error", "-rtsp_transport", "tcp", "-timeout", "8000000", "-read_intervals", "%+2", "-count_packets", "-show_entries", "stream=codec_type,codec_name,width,height,r_frame_rate,has_b_frames,nb_read_packets", "-of", "json", source)
	cmd.Stdout = &out
	cmd.Stderr = &diagnostics
	if e := cmd.Run(); e != nil {
		if ctx.Err() != nil {
			return model.Probe{}, ctx.Err()
		}
		text := strings.ToLower(diagnostics.String())
		code, message := "SOURCE_UNREACHABLE", "Stream tidak terbaca. Periksa IP, port RTSP, path, koneksi dan firewall"
		if strings.Contains(text, "401") || strings.Contains(text, "403") || strings.Contains(text, "unauthorized") {
			code, message = "DEVICE_AUTH_FAILED", "Kredensial RTSP ditolak. Akun RTSP dan ONVIF dapat berbeda"
		}
		if _, e := exec.LookPath(f.ProbeBinary); e != nil {
			code, message = "BINARY_MISSING", "ffprobe tidak ditemukan; periksa paket aplikasi"
		}
		return model.Probe{}, &model.Failure{Code: code, Message: message}
	}
	var data struct {
		Streams []struct {
			Type    string `json:"codec_type"`
			Codec   string `json:"codec_name"`
			Width   int    `json:"width"`
			Height  int    `json:"height"`
			Rate    string `json:"r_frame_rate"`
			BFrames int    `json:"has_b_frames"`
			Packets string `json:"nb_read_packets"`
		} `json:"streams"`
	}
	if json.Unmarshal(out.Bytes(), &data) != nil {
		return model.Probe{}, &model.Failure{Code: "INVALID_MEDIA", Message: "Informasi stream tidak valid"}
	}
	result := model.Probe{TestedAt: time.Now().UTC()}
	for _, s := range data.Streams {
		packets, _ := strconv.Atoi(s.Packets)
		if packets < 1 {
			continue
		}
		if s.Type == "video" {
			result.VideoCodec = s.Codec
			result.Width = s.Width
			result.Height = s.Height
			result.BFrames = s.BFrames
			parts := strings.Split(s.Rate, "/")
			if len(parts) == 2 {
				a, _ := strconv.ParseFloat(parts[0], 64)
				b, _ := strconv.ParseFloat(parts[1], 64)
				if b > 0 {
					result.FPS = a / b
				}
			}
		}
		if s.Type == "audio" {
			result.HasAudio = true
			result.AudioCodec = s.Codec
		}
	}
	if result.VideoCodec == "" {
		return result, &model.Failure{Code: "NO_VIDEO", Message: "Tidak ada paket video pada profil ini"}
	}
	return result, model.ValidateMedia(result)
}
func (f Inspector) Check(ctx context.Context) []model.Diagnostic {
	results := []model.Diagnostic{}
	for _, v := range []struct{ bin, name string }{{f.Binary, "FFmpeg"}, {f.ProbeBinary, "ffprobe"}, {f.MediaBinary, "MediaMTX"}} {
		code, message := "OK", v.name+" tersedia"
		if _, e := exec.LookPath(v.bin); e != nil {
			code, message = "BINARY_MISSING", v.name+" tidak ditemukan"
		}
		results = append(results, model.Diagnostic{Component: v.name, Code: code, Message: message})
	}
	return results
}
