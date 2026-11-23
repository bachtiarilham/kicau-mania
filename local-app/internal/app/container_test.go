package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kicau.local/edge/config"
)

func TestNewMediaMapsChannelSnapshotWithoutStartingProcesses(t *testing.T) {
	cfg := config.Config{
		MediaMTX:    "missing-executable",
		MediaConfig: filepath.Join(t.TempDir(), "mediamtx.yml"),
		RTSPBase:    "rtsp://127.0.0.1:8554",
		RTSPListen:  "127.0.0.1:8554",
		Origins:     []string{"http://arena.test"},
		ReadCIDRs:   []string{"192.0.2.0/24"},
		Channels:    []config.Channel{{ID: "one", Source: "rtsp://operator:secret@192.0.2.1/feed"}},
	}
	catalog, server, err := NewMedia(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if server.Done() != nil {
		t.Fatal("builder started a media process")
	}
	if _, err := os.Stat(cfg.MediaConfig); !os.IsNotExist(err) {
		t.Fatal("builder rendered configuration before requested")
	}
	// Mutating caller configuration cannot change the catalog or server snapshot.
	cfg.Channels[0] = config.Channel{ID: "changed", Source: "rtsp://192.0.2.99/changed"}
	if got, err := catalog.HubRTSP("one"); err != nil || got != "rtsp://127.0.0.1:8554/one" {
		t.Fatalf("local channel URL: %q %v", got, err)
	}
	if err := server.Render(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(cfg.MediaConfig)
	if err != nil {
		t.Fatal(err)
	}
	var rendered struct {
		Paths map[string]struct {
			Source      string `json:"source"`
			RunOnDemand string `json:"runOnDemand"`
		} `json:"paths"`
		UDP    string `json:"webrtcLocalUDPAddress"`
		WebRTC bool   `json:"webrtc"`
	}
	if err := json.Unmarshal(b, &rendered); err != nil {
		t.Fatal(err)
	}
	if rendered.WebRTC || len(rendered.Paths) != 2 || rendered.Paths["one"].Source != "rtsp://operator:secret@192.0.2.1/feed" {
		t.Fatal("builder lost upstream mapping")
	}
	if server.Done() != nil {
		t.Fatal("render started a media process")
	}
	zoom := rendered.Paths["zoom/one_zoom"]
	if zoom.Source != "publisher" || !strings.Contains(zoom.RunOnDemand, "rtsp://127.0.0.1:8554/zoom/one_zoom") || strings.Contains(zoom.RunOnDemand, "secret") {
		t.Fatal("zoom must publish from the local hub without upstream credentials")
	}
	server.Close()
}
