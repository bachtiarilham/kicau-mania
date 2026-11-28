package app

import (
	"testing"

	"kicau.local/edge/internal/camera/repository"
)

func TestArenaProbeUsesRelayOnlyForActiveSource(t *testing.T) {
	const saved = "rtsp://user:pass@192.0.2.1:554/video1"
	const relay = "rtsp://127.0.0.1:8554/gantangan01"
	catalog, err := repository.New([]repository.Input{{ID: "gantangan01", Source: saved}}, "rtsp://127.0.0.1:8554")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                  string
		catalog               *repository.Catalog
		id, source, want, via string
	}{
		{"applied camera", catalog, "gantangan01", saved, relay, "mediamtx"},
		{"edited path", catalog, "gantangan01", saved + "-new", saved + "-new", "direct"},
		{"edited credentials", catalog, "gantangan01", "rtsp://user:new@192.0.2.1:554/video1", "rtsp://user:new@192.0.2.1:554/video1", "direct"},
		{"new slot", catalog, "gantangan02", saved, saved, "direct"},
		{"unconfigured engine", nil, "gantangan01", saved, saved, "direct"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if target, via := arenaProbeTarget(tc.catalog, tc.id, tc.source); target != tc.want || via != tc.via {
				t.Fatal("unexpected camera probe route")
			}
		})
	}
}
