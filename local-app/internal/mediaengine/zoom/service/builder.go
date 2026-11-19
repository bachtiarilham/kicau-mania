package service

import (
	"fmt"
	"strings"

	"kicau.local/edge/internal/mediaengine/zoom/model"
)

type Builder struct {
	FFmpegBinary string
}

func NewBuilder(ffmpegBinary string) *Builder {
	if ffmpegBinary == "" {
		ffmpegBinary = "ffmpeg"
	}
	return &Builder{FFmpegBinary: ffmpegBinary}
}

// BuildFilterGraph generates the FFmpeg video filter chain for crop, perspective warp, and unsharp.
func (b *Builder) BuildFilterGraph(ch model.ZoomChannel) string {
	filters := []string{
		fmt.Sprintf("crop=%d:%d:%d:%d", ch.Crop.Width, ch.Crop.Height, ch.Crop.X, ch.Crop.Y),
	}

	if ch.Perspective.Enabled {
		pts := ch.Perspective.SrcPoints
		pFilter := fmt.Sprintf("perspective=x0=%d:y0=%d:x1=%d:y1=%d:x2=%d:y2=%d:x3=%d:y3=%d:interpol=linear",
			pts[0][0], pts[0][1],
			pts[1][0], pts[1][1],
			pts[2][0], pts[2][1],
			pts[3][0], pts[3][1],
		)
		filters = append(filters, pFilter)
	}

	// CAS / Unsharp sharpening filter for feather and eye clarity
	filters = append(filters, "unsharp=5:5:1.0:5:5:0.0")

	return strings.Join(filters, ",")
}

// BuildArgs constructs the full argument slice for hardware-accelerated zero-delay zoom streaming.
func (b *Builder) BuildArgs(ch model.ZoomChannel, sourceRTSP, targetRTSP string) []string {
	filterGraph := b.BuildFilterGraph(ch)

	args := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-hwaccel", "cuda",
		"-rtsp_transport", "tcp",
		"-i", sourceRTSP,
		"-filter_complex", fmt.Sprintf("[0:v]%s[outv]", filterGraph),
		"-map", "[outv]",
		"-an",
		"-c:v", "hevc_nvenc",
		"-preset", "p1",
		"-tune", "ull",
		"-zerolatency", "1",
		"-rc", "cbr",
		"-b:v", "2500k",
		"-maxrate", "2500k",
		"-bufsize", "500k",
		"-g", "25",
		"-bf", "0",
		"-f", "rtsp",
		"-rtsp_transport", "tcp",
		targetRTSP,
	}

	return args
}
