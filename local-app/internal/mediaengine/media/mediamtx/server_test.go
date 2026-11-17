package mediamtx

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kicau.local/edge/internal/mediaengine/media"
)

func TestGeneratedConfigPreservesUpstreamMapping(t *testing.T) {
	sources := []Source{{ID: "one", URL: "rtsp://user:secret@192.0.2.1/one"}, {ID: "two", URL: "rtsp://192.0.2.1/two"}}
	var e error
	opts := Options{ConfigFile: filepath.Join(t.TempDir(), "data", "mediamtx.yml"), RTSPListen: "127.0.0.1:8554", ReadCIDRs: []string{"192.0.2.0/24"}}
	mediaServer := newTestServer(opts, sources)
	if e = mediaServer.Render(); e != nil {
		t.Fatal(e)
	}
	if mediaServer.Done() != nil {
		t.Fatal("render started MediaMTX")
	}
	b, e := os.ReadFile(opts.ConfigFile)
	if e != nil {
		t.Fatal(e)
	}
	var cfg map[string]any
	if e = json.Unmarshal(b, &cfg); e != nil {
		t.Fatal(e)
	}
	paths := cfg["paths"].(map[string]any)
	if paths["one"].(map[string]any)["source"] != "rtsp://user:secret@192.0.2.1/one" || paths["two"].(map[string]any)["source"] != "rtsp://192.0.2.1/two" {
		t.Fatal(paths)
	}
	if len(paths) != 2 || cfg["api"] != false || cfg["hls"] != false || cfg["webrtc"] != false {
		t.Fatal(cfg)
	}
	direct := paths["one"].(map[string]any)
	if direct["runOnDemand"] != nil || direct["runOnInit"] != nil {
		t.Fatal("HEVC passthrough must not spawn FFmpeg")
	}
	users := cfg["authInternalUsers"].([]any)
	if len(users) != 1 {
		t.Fatal("media ACL changed")
	}
	mediaServer.Close()
	mediaServer.Close()
}

func TestFailedStartDoesNotRetainProcess(t *testing.T) {
	sources := []Source{{ID: "one", URL: "rtsp://192.0.2.1/a"}}
	var e error
	mediaServer := newTestServer(Options{Binary: filepath.Join(t.TempDir(), "missing-executable"), ConfigFile: filepath.Join(t.TempDir(), "mediamtx.yml")}, sources)
	if e = mediaServer.Start(context.Background()); e == nil {
		mediaServer.Close()
		t.Fatal("missing binary started")
	}
	if mediaServer.Done() != nil {
		t.Fatal("failed start retained process")
	}
	mediaServer.Close()
}

type trackedProcess struct{ done chan struct{} }

func (p *trackedProcess) Done() <-chan struct{} { return p.done }
func (p *trackedProcess) Err() error            { <-p.done; return nil }
func (p *trackedProcess) Stop()                 {}
func TestServerCloseCancelsAndWaitsForChild(t *testing.T) {
	sources := []Source{{ID: "one", URL: "rtsp://192.0.2.1/a"}}
	var e error
	opts := Options{Binary: "mediamtx", ConfigFile: filepath.Join(t.TempDir(), "mediamtx.yml")}
	mediaServer := newTestServer(opts, sources)
	p := &trackedProcess{done: make(chan struct{})}
	mediaServer.launch = func(ctx context.Context, binary string, args []string) (media.Process, error) {
		if binary != opts.Binary || len(args) != 1 || args[0] != opts.ConfigFile {
			t.Fatal("incorrect child arguments")
		}
		child := p
		go func() { <-ctx.Done(); close(child.done) }()
		return child, nil
	}
	if e = mediaServer.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = mediaServer.Start(context.Background()); e == nil {
		t.Fatal("duplicate mediaServer started")
	}
	closed := make(chan struct{})
	go func() { mediaServer.Close(); mediaServer.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("mediaServer shutdown leaked child")
	}
	select {
	case <-p.Done():
	default:
		t.Fatal("Close returned before child finished")
	}
	if mediaServer.Done() != nil {
		t.Fatal("closed mediaServer retained process")
	}
	p = &trackedProcess{done: make(chan struct{})}
	if e = mediaServer.Start(context.Background()); e != nil {
		t.Fatalf("restart after Close: %v", e)
	}
	mediaServer.Close()
}

func TestRenderEmptyReadCIDRsKeepsOnlyLoopback(t *testing.T) {
	for _, cidrs := range [][]string{nil, {}} {
		sources := []Source{{ID: "one", URL: "rtsp://192.0.2.1/a"}}
		path := filepath.Join(t.TempDir(), "mediamtx.yml")
		mediaServer := newTestServer(Options{ConfigFile: path, ReadCIDRs: cidrs}, sources)
		if err := mediaServer.Render(); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var cfg struct {
			Users []struct {
				IPs []string `json:"ips"`
			} `json:"authInternalUsers"`
			UDP string `json:"webrtcLocalUDPAddress"`
		}
		if err := json.Unmarshal(b, &cfg); err != nil {
			t.Fatal(err)
		}
		if len(cfg.Users) != 1 || len(cfg.Users[0].IPs) != 2 || cfg.Users[0].IPs[0] != "127.0.0.1/32" || cfg.Users[0].IPs[1] != "::1/128" {
			t.Fatal("empty CIDRs must not create unrestricted read access")
		}
	}
}

func TestStartPreservesLaunchErrorAndAllowsRetry(t *testing.T) {
	sources := []Source{{ID: "one", URL: "rtsp://192.0.2.1/a"}}
	mediaServer := newTestServer(Options{ConfigFile: filepath.Join(t.TempDir(), "mediamtx.yml")}, sources)
	want := errors.New("launch failed")
	var failedContext context.Context
	mediaServer.launch = func(ctx context.Context, _ string, _ []string) (media.Process, error) {
		failedContext = ctx
		return nil, want
	}
	if err := mediaServer.Start(context.Background()); !errors.Is(err, want) {
		t.Fatalf("launch error lost: %v", err)
	}
	if failedContext.Err() != context.Canceled {
		t.Fatal("failed launch context was not canceled")
	}
	mediaServer.launch = func(ctx context.Context, _ string, _ []string) (media.Process, error) {
		p := &trackedProcess{done: make(chan struct{})}
		go func() { <-ctx.Done(); close(p.done) }()
		return p, nil
	}
	if err := mediaServer.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	mediaServer.Close()
}

func newTestServer(opts Options, sources []Source) *Server {
	opts.Sources = sources
	return New(opts)
}

func TestServerSnapshotsSourceOptions(t *testing.T) {
	sources := []Source{{ID: "one", URL: "rtsp://192.0.2.1/feed"}}
	path := filepath.Join(t.TempDir(), "mediamtx.yml")
	server := New(Options{ConfigFile: path, Sources: sources})
	sources[0] = Source{ID: "changed", URL: "rtsp://192.0.2.99/changed"}
	if err := server.Render(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Paths map[string]struct {
			Source string `json:"source"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Paths) != 1 || cfg.Paths["one"].Source != "rtsp://192.0.2.1/feed" {
		t.Fatal("caller mutation changed the server source snapshot")
	}
}
