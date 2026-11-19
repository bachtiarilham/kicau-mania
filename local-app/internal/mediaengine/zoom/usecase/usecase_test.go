package usecase

import (
	"context"
	"testing"

	"kicau.local/edge/internal/mediaengine/zoom/model"
	"kicau.local/edge/internal/mediaengine/zoom/service"
)

type memoryRepo struct {
	mapping model.ZoomMapping
}

func (m *memoryRepo) Load() (model.ZoomMapping, error) {
	return m.mapping, nil
}

func (m *memoryRepo) Save(mapping model.ZoomMapping) error {
	m.mapping = mapping
	return nil
}

func TestUseCaseListAndCalibration(t *testing.T) {
	repo := &memoryRepo{
		mapping: model.ZoomMapping{
			ActiveChannels: []model.ZoomChannel{
				{
					ChannelID:    "gantangan_22",
					Area:         "Area A",
					Active:       true,
					SourceCamera: "cam1",
					Crop:         model.CropConfig{X: 100, Y: 100, Width: 400, Height: 400},
				},
				{
					ChannelID:    "gantangan_dummy",
					Area:         "Area A",
					Active:       false,
					SourceCamera: "cam1",
					Crop:         model.CropConfig{X: 0, Y: 0, Width: 100, Height: 100},
				},
			},
		},
	}

	uc, err := New(repo, service.NewBuilder("ffmpeg"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	// Ensure only active channels are returned
	active := uc.ListActiveChannels(ctx)
	if len(active) != 1 || active[0].ChannelID != "gantangan_22" {
		t.Fatalf("expected 1 active channel, got %d", len(active))
	}

	// Update calibration
	newCrop := model.CropConfig{X: 200, Y: 250, Width: 500, Height: 500}
	newPersp := model.PerspectiveConfig{
		Enabled:   true,
		SrcPoints: [4]model.Point{{10, 10}, {500, 20}, {15, 490}, {510, 505}},
	}

	if err := uc.UpdateCalibration(ctx, "gantangan_22", newCrop, newPersp); err != nil {
		t.Fatalf("failed to update calibration: %v", err)
	}

	ch, err := uc.GetChannel(ctx, "gantangan_22")
	if err != nil {
		t.Fatalf("failed to get channel: %v", err)
	}

	if ch.Crop.X != 200 || ch.Crop.Width != 500 || !ch.Perspective.Enabled {
		t.Errorf("calibration did not update properly: %+v", ch)
	}

	// Test building FFmpeg args
	args, err := uc.BuildArgs(ctx, "gantangan_22", "rtsp://127.0.0.1:8554")
	if err != nil {
		t.Fatalf("failed to build args: %v", err)
	}

	if len(args) == 0 {
		t.Fatal("expected non-empty args")
	}
}
