package service

import (
	"fmt"
	"strings"

	"kicau.local/edge/internal/camera/model"
)

type Builder struct {
	FFmpegBinary string
	AudioFilter  string
}

func NewBuilder(ffmpegBinary string) *Builder {
	if ffmpegBinary == "" {
		ffmpegBinary = "ffmpeg"
	}
	return &Builder{FFmpegBinary: ffmpegBinary}
}

func (b *Builder) BuildFilterGraph(ch model.ZoomChannel) string {
	filters := []string{
		fmt.Sprintf("crop=%d:%d:%d:%d", ch.Crop.Width, ch.Crop.Height, ch.Crop.X, ch.Crop.Y),
	}

	if ch.Perspective.Enabled {
		pts := ch.Perspective.SrcPoints
		for i := range pts {
			pts[i][0] -= ch.Crop.X
			pts[i][1] -= ch.Crop.Y
		}
		pFilter := fmt.Sprintf("perspective=x0=%d:y0=%d:x1=%d:y1=%d:x2=%d:y2=%d:x3=%d:y3=%d:interpol=linear",
			pts[0][0], pts[0][1],
			pts[1][0], pts[1][1],
			pts[2][0], pts[2][1],
			pts[3][0], pts[3][1],
		)
		filters = append(filters, pFilter)
	}

	filters = append(filters, "unsharp=5:5:1.0:5:5:0.0")
	return strings.Join(filters, ",")
}

func (b *Builder) BuildArgs(ch model.ZoomChannel, sourceRTSP, targetRTSP string) []string {
	filter := "crop=trunc(iw/4)*2:trunc(ih/4)*2,scale=iw*2:ih*2"
	if ch.Crop.Width > 0 {
		filter = b.BuildFilterGraph(ch) + ",scale=trunc(iw/2)*2:trunc(ih/2)*2"
	}

	args := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", "error",
		"-rtsp_transport", "tcp",
		"-timeout", "10000000",
		"-i", sourceRTSP,
		"-map", "0:v:0", "-map", "0:a:0?", "-vf", filter,
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", "ultrafast",
		"-tune", "zerolatency", "-c:a", "aac", "-b:a", "128k",
		"-g", "25",
		"-bf", "0",
	}
	if b.AudioFilter != "" {
		args = append(args, "-af", b.AudioFilter)
	}
	return append(args, "-f", "rtsp", "-rtsp_transport", "tcp", targetRTSP)
}
