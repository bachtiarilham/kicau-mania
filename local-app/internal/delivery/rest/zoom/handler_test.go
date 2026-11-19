package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/mediaengine/zoom/model"
)

type mockZoomUC struct {
	channels []model.ZoomChannel
}

func (m *mockZoomUC) ListActiveChannels(ctx context.Context) []model.ZoomChannel {
	return m.channels
}

func (m *mockZoomUC) GetChannel(ctx context.Context, id string) (model.ZoomChannel, error) {
	for _, ch := range m.channels {
		if ch.ChannelID == id {
			return ch, nil
		}
	}
	return model.ZoomChannel{}, model.ErrInvalidChannel
}

func (m *mockZoomUC) UpdateCalibration(ctx context.Context, id string, crop model.CropConfig, perspective model.PerspectiveConfig) error {
	for i, ch := range m.channels {
		if ch.ChannelID == id {
			m.channels[i].Crop = crop
			m.channels[i].Perspective = perspective
			return nil
		}
	}
	return model.ErrInvalidChannel
}

func TestZoomHandlerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mock := &mockZoomUC{
		channels: []model.ZoomChannel{
			{
				ChannelID:    "gantangan_22",
				Area:         "Area A",
				Active:       true,
				SourceCamera: "cam_side_nw",
				Crop:         model.CropConfig{X: 100, Y: 100, Width: 400, Height: 400},
			},
		},
	}

	h := New(mock)
	r := gin.New()
	g := r.Group("/api/v1")
	h.Routes(g)

	t.Run("GET /api/v1/zoom/channels", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/zoom/channels", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("GET /api/v1/zoom/channels/:id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/zoom/channels/gantangan_22", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})

	t.Run("PUT /api/v1/zoom/channels/:id/calibration", func(t *testing.T) {
		body, _ := json.Marshal(CalibrationRequest{
			Crop: model.CropConfig{X: 150, Y: 150, Width: 450, Height: 450},
			Perspective: model.PerspectiveConfig{
				Enabled: true,
				SrcPoints: [4]model.Point{
					{10, 10}, {400, 20}, {15, 390}, {410, 405},
				},
			},
		})

		req := httptest.NewRequest(http.MethodPut, "/api/v1/zoom/channels/gantangan_22/calibration", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
		}
	})
}
