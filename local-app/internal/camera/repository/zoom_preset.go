package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"kicau.local/edge/internal/camera/model"
)

type FileRepository struct {
	mu   sync.RWMutex
	path string
}

func NewFileRepository(path string) *FileRepository {
	return &FileRepository{path: path}
}

func (r *FileRepository) Load() (model.ZoomMapping, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultWhiteNumberedGantangan(), nil
		}
		return model.ZoomMapping{}, err
	}

	var m model.ZoomMapping
	if err := json.Unmarshal(data, &m); err != nil {
		return model.ZoomMapping{}, err
	}
	return m, nil
}

func (r *FileRepository) Save(m model.ZoomMapping) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}

func defaultWhiteNumberedGantangan() model.ZoomMapping {
	channels := []model.ZoomChannel{
		{ChannelID: "gantangan_37", Area: "Area A", Active: true, SourceCamera: "cam_side_nw", LineOfSight: "Sudut Barat Laut", Crop: model.CropConfig{X: 100, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_36", Area: "Area A", Active: true, SourceCamera: "cam_side_north", LineOfSight: "Sisi Utara Tegak", Crop: model.CropConfig{X: 700, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_35", Area: "Area A", Active: true, SourceCamera: "cam_side_north", LineOfSight: "Sisi Utara Tegak", Crop: model.CropConfig{X: 1300, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_26", Area: "Area A", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Celah", Crop: model.CropConfig{X: 700, Y: 700, Width: 600, Height: 600}},
		{ChannelID: "gantangan_27", Area: "Area A", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Diagonal", Crop: model.CropConfig{X: 1300, Y: 700, Width: 600, Height: 600}, Perspective: model.PerspectiveConfig{Enabled: true, SrcPoints: [4]model.Point{{1320, 710}, {1890, 730}, {1310, 1290}, {1880, 1310}}}},
		{ChannelID: "gantangan_24", Area: "Area A", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Tegak", Crop: model.CropConfig{X: 100, Y: 1300, Width: 600, Height: 600}},
		{ChannelID: "gantangan_23", Area: "Area A", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Celah", Crop: model.CropConfig{X: 700, Y: 1300, Width: 600, Height: 600}},
		{ChannelID: "gantangan_22", Area: "Area A", Active: true, SourceCamera: "cam_side_nw", LineOfSight: "Diagonal Celah 37-25", Crop: model.CropConfig{X: 1420, Y: 850, Width: 640, Height: 640}, Perspective: model.PerspectiveConfig{Enabled: true, SrcPoints: [4]model.Point{{1450, 860}, {2040, 880}, {1430, 1460}, {2020, 1480}}}},
		{ChannelID: "gantangan_34", Area: "Area B", Active: true, SourceCamera: "cam_side_north", LineOfSight: "Sisi Utara Tegak", Crop: model.CropConfig{X: 200, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_32", Area: "Area B", Active: true, SourceCamera: "cam_side_north", LineOfSight: "Sisi Utara Tegak", Crop: model.CropConfig{X: 800, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_31", Area: "Area B", Active: true, SourceCamera: "cam_side_ne", LineOfSight: "Sudut Timur Laut", Crop: model.CropConfig{X: 1400, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_28", Area: "Area B", Active: true, SourceCamera: "cam_side_east", LineOfSight: "Sisi Timur Diagonal", Crop: model.CropConfig{X: 200, Y: 700, Width: 600, Height: 600}, Perspective: model.PerspectiveConfig{Enabled: true, SrcPoints: [4]model.Point{{220, 710}, {790, 730}, {210, 1290}, {780, 1310}}}},
		{ChannelID: "gantangan_21", Area: "Area B", Active: true, SourceCamera: "cam_side_ne", LineOfSight: "Diagonal Celah 31-30", Crop: model.CropConfig{X: 200, Y: 1300, Width: 640, Height: 640}, Perspective: model.PerspectiveConfig{Enabled: true, SrcPoints: [4]model.Point{{220, 1310}, {810, 1330}, {210, 1910}, {800, 1930}}}},
		{ChannelID: "gantangan_19", Area: "Area B", Active: true, SourceCamera: "cam_side_east", LineOfSight: "Sisi Timur Tegak", Crop: model.CropConfig{X: 1400, Y: 1300, Width: 600, Height: 600}},
		{ChannelID: "gantangan_13", Area: "Area C", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Tegak", Crop: model.CropConfig{X: 100, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_14", Area: "Area C", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Celah", Crop: model.CropConfig{X: 700, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_12", Area: "Area C", Active: true, SourceCamera: "cam_side_west", LineOfSight: "Sisi Barat Tegak", Crop: model.CropConfig{X: 100, Y: 700, Width: 600, Height: 600}},
		{ChannelID: "gantangan_01", Area: "Area C", Active: true, SourceCamera: "cam_side_sw", LineOfSight: "Sudut Barat Daya", Crop: model.CropConfig{X: 100, Y: 1300, Width: 600, Height: 600}},
		{ChannelID: "gantangan_18", Area: "Area D", Active: true, SourceCamera: "cam_side_east", LineOfSight: "Sisi Timur Tegak", Crop: model.CropConfig{X: 1400, Y: 100, Width: 600, Height: 600}},
		{ChannelID: "gantangan_06", Area: "Area D", Active: true, SourceCamera: "cam_side_se", LineOfSight: "Sudut Tenggara", Crop: model.CropConfig{X: 1400, Y: 1300, Width: 600, Height: 600}},
	}
	return model.ZoomMapping{ActiveChannels: channels}
}
