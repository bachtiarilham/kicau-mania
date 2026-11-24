package service

import (
	"strings"
	"testing"

	"kicau.local/edge/internal/camera/model"
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

	t.Run("diagonal crop with perspective warp", func(t *testing.T) {
		ch := model.ZoomChannel{
			ChannelID: "gantangan_27",
			Crop:      model.CropConfig{X: 1300, Y: 700, Width: 600, Height: 600},
			Perspective: model.PerspectiveConfig{
				Enabled:   true,
				SrcPoints: [4]model.Point{{1320, 710}, {1890, 730}, {1310, 1290}, {1880, 1310}},
			},
		}

		filter := b.BuildFilterGraph(ch)
		expectedPerspective := "perspective=x0=20:y0=10:x1=590:y1=30:x2=10:y2=590:x3=580:y3=610:interpol=linear"
		if !strings.Contains(filter, expectedPerspective) {
			t.Errorf("expected %s in filter, got %s", expectedPerspective, filter)
		}
	})
}

func TestBuildArgs(t *testing.T) {
	b := NewBuilder("custom_ffmpeg")
	b.AudioFilter = "volume=1.5"

	ch := model.ZoomChannel{
		ChannelID: "gantangan_01",
		Crop:      model.CropConfig{X: 100, Y: 100, Width: 400, Height: 400},
	}

	args := b.BuildArgs(ch, "rtsp://127.0.0.1:8554/cam_side_sw", "rtsp://127.0.0.1:8554/gantangan_01_zoom")
	cmdStr := strings.Join(args, " ")

	if !strings.Contains(cmdStr, "-i rtsp://127.0.0.1:8554/cam_side_sw") {
		t.Errorf("missing input RTSP in args: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "rtsp://127.0.0.1:8554/gantangan_01_zoom") {
		t.Errorf("missing target RTSP in args: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-af volume=1.5") {
		t.Errorf("expected audio filter in args: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "-c:v libx264") {
		t.Errorf("expected h264 encoder in args: %s", cmdStr)
	}
}
