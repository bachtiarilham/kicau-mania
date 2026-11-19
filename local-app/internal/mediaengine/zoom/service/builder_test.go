package service

import (
	"strings"
	"testing"

	"kicau.local/edge/internal/mediaengine/zoom/model"
)

func TestBuildFilterGraph(t *testing.T) {
	b := NewBuilder("ffmpeg")

	t.Run("perpendicular crop without perspective", func(t *testing.T) {
		ch := model.ZoomChannel{
			ChannelID: "gantangan_36",
			Crop:      model.CropConfig{X: 800, Y: 200, Width: 500, Height: 500},
			Perspective: model.PerspectiveConfig{
				Enabled: false,
			},
		}

		filter := b.BuildFilterGraph(ch)
		if !strings.Contains(filter, "crop=500:500:800:200") {
			t.Errorf("expected crop in filter, got %s", filter)
		}
		if strings.Contains(filter, "perspective=") {
			t.Errorf("did not expect perspective filter when disabled, got %s", filter)
		}
		if !strings.Contains(filter, "unsharp=5:5:1.0:5:5:0.0") {
			t.Errorf("expected unsharp filter, got %s", filter)
		}
	})

	t.Run("diagonal angle with perspective warp", func(t *testing.T) {
		ch := model.ZoomChannel{
			ChannelID: "gantangan_22",
			Crop:      model.CropConfig{X: 1420, Y: 850, Width: 640, Height: 640},
			Perspective: model.PerspectiveConfig{
				Enabled: true,
				SrcPoints: [4]model.Point{
					{1450, 860},
					{2040, 880},
					{1430, 1460},
					{2020, 1480},
				},
			},
		}

		filter := b.BuildFilterGraph(ch)
		if !strings.Contains(filter, "crop=640:640:1420:850") {
			t.Errorf("expected crop, got %s", filter)
		}
		if !strings.Contains(filter, "perspective=x0=1450:y0=860:x1=2040:y1=880:x2=1430:y2=1460:x3=2020:y3=1480") {
			t.Errorf("expected perspective coordinates, got %s", filter)
		}
	})
}

func TestBuildArgsZeroLatency(t *testing.T) {
	b := NewBuilder("ffmpeg")
	ch := model.ZoomChannel{
		ChannelID: "gantangan_22",
		Crop:      model.CropConfig{X: 100, Y: 100, Width: 400, Height: 400},
	}

	args := b.BuildArgs(ch, "rtsp://127.0.0.1:8554/cam1", "rtsp://127.0.0.1:8554/gantangan_22_zoom")
	full := strings.Join(args, " ")

	expectedFlags := []string{
		"-hwaccel cuda",
		"-c:v hevc_nvenc",
		"-preset p1",
		"-tune ull",
		"-zerolatency 1",
		"-bf 0",
		"-g 25",
		"-f rtsp",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(full, flag) {
			t.Errorf("expected flag %q in args, got: %s", flag, full)
		}
	}
}
