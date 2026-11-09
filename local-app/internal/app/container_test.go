package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"kicau.local/edge/config"
)

func TestNewMediaMapsChannelSnapshotWithoutStartingProcesses(t *testing.T) {
	cfg := config.Config{
		MediaMTX:       "missing-executable",
		MediaConfig:    filepath.Join(t.TempDir(), "mediamtx.yml"),
		RTSPBase:       "rtsp://127.0.0.1:8554",
		RTSPListen:     "127.0.0.1:8554",
		WebRTCListen:   ":8889",
		WebRTCLocalUDP: ":8190",
		WebRTCBase:     "http://arena.test:8889",
		Origins:        []string{"http://arena.test"},
		ReadCIDRs:      []string{"192.0.2.0/24"},
		Channels:       []config.Channel{{ID: "one", Source: "rtsp://operator:secret@192.0.2.1/feed"}},
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
	if server.WHEP("one") != "http://arena.test:8889/api/v1/playback/one/whep" {
		t.Fatal("WebRTC URL changed")
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
			Source string `json:"source"`
		} `json:"paths"`
		UDP string `json:"webrtcLocalUDPAddress"`
	}
	if err := json.Unmarshal(b, &rendered); err != nil {
		t.Fatal(err)
	}
	if len(rendered.Paths) != 1 || rendered.Paths["one"].Source != "rtsp://operator:secret@192.0.2.1/feed" || rendered.UDP != ":8190" {
		t.Fatal("builder lost upstream mapping or UDP configuration")
	}
	if server.Done() != nil {
		t.Fatal("render started a media process")
	}
	server.Close()
}
