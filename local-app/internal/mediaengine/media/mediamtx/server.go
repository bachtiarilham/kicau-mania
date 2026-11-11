// Package mediamtx configures and manages the local MediaMTX server.
package mediamtx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/mediaengine/media"
)

type Source struct {
	ID, URL string
}

type Options struct {
	PublishToken                                             string
	Sources                                                  []Source
	Binary, ConfigFile, RTSPListen, WebRTCListen, WebRTCBase string
	WebRTCLocalUDP                                           string
	ICEHosts, Origins, ReadCIDRs                             []string
}
type Server struct {
	opts   Options
	mu     sync.Mutex
	p      media.Process
	cancel context.CancelFunc
	launch func(context.Context, string, []string) (media.Process, error)
}

func New(opts Options) *Server {
	if opts.WebRTCLocalUDP == "" {
		opts.WebRTCLocalUDP = ":8189"
	}
	opts.Sources = append([]Source(nil), opts.Sources...)
	return &Server{opts: opts, launch: func(ctx context.Context, binary string, args []string) (media.Process, error) {
		return subprocess.StartMediaServer(ctx, binary, args, nil)
	}}
}
func (s *Server) WHEP(id string) string {
	return strings.TrimRight(s.opts.WebRTCBase, "/") + "/api/v1/playback/" + id + "/whep"
}

func (s *Server) MediaStatus() media.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return media.Status{State: "UNAVAILABLE"}
	}
	select {
	case <-s.p.Done():
		return media.Status{State: "UNAVAILABLE"}
	default:
		return media.Status{State: "RUNNING"}
	}
}

// Render keeps the existing JSON-compatible YAML schema and filesystem location.
func (s *Server) Render() error {
	paths := map[string]any{}
	for _, in := range s.opts.Sources {
		if in.URL == "" {
			paths[in.ID] = map[string]any{"source": "publisher", "overridePublisher": false}
			continue
		}
		paths[in.ID] = map[string]any{
			"source":                   in.URL,
			"rtspTransport":            "tcp",
			"sourceOnDemand":           true,
			"sourceOnDemandCloseAfter": "1s",
		}
	}
	authUsers := []any{
		map[string]any{
			"user": "any",
			"ips":  []string{"127.0.0.1/32", "::1/128"},
			"permissions": []any{
				map[string]any{"action": "read", "path": ""},
			}},
	}
	if s.opts.PublishToken != "" {
		authUsers = append(authUsers, map[string]any{"user": "pipeline", "pass": s.opts.PublishToken, "ips": []string{"127.0.0.1/32", "::1/128"}, "permissions": []any{map[string]any{"action": "publish", "path": ""}}})
	}
	cfg := map[string]any{
		"logLevel":                "warn",
		"rtsp":                    true,
		"rtspAddress":             s.opts.RTSPListen,
		"rtspTransports":          []string{"tcp"},
		"rtmp":                    false,
		"hls":                     false,
		"srt":                     false,
		"api":                     false,
		"playback":                false,
		"webrtc":                  true,
		"webrtcAddress":           s.opts.WebRTCListen,
		"webrtcLocalUDPAddress":   s.opts.WebRTCLocalUDP,
		"webrtcAdditionalHosts":   s.opts.ICEHosts,
		"webrtcIPsFromInterfaces": false,
		"webrtcICEServers2":       []any{},
		"webrtcTrustedProxies":    []string{"127.0.0.1/32", "::1/128"},
		"webrtcAllowOrigin":       s.webRTCOrigin(),
		"authMethod":              "internal",
		"authInternalUsers":       authUsers,
		"paths":                   paths,
	}
	b, e := json.MarshalIndent(cfg, "", "  ")
	if e != nil {
		return fmt.Errorf("marshal MediaMTX config: %w", e)
	}
	if e = os.MkdirAll(filepath.Dir(s.opts.ConfigFile), 0700); e != nil {
		return fmt.Errorf("create MediaMTX config directory: %w", e)
	}
	if e = os.WriteFile(s.opts.ConfigFile, b, 0600); e != nil {
		return fmt.Errorf("write MediaMTX config: %w", e)
	}
	return nil
}

// MediaMTX v1.15.3 accepts one CORS origin. Go API CORS still uses the full list.
func (s *Server) webRTCOrigin() string {
	if len(s.opts.Origins) > 0 {
		return s.opts.Origins[0]
	}
	return "http://localhost:3000"
}
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p != nil {
		return fmt.Errorf("MediaMTX already started")
	}
	if e := s.Render(); e != nil {
		return e
	}
	child, cancel := context.WithCancel(ctx)
	p, e := s.launch(child, s.opts.Binary, []string{s.opts.ConfigFile})
	if e != nil {
		cancel()
		return fmt.Errorf("cannot start MediaMTX: %w", e)
	}
	s.p = p
	s.cancel = cancel
	return nil
}
func (s *Server) Done() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.p == nil {
		return nil
	}
	return s.p.Done()
}
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Serialize shutdown and Start so the old process releases its ports first.
	if s.p != nil {
		s.cancel()
		<-s.p.Done()
		s.p = nil
		s.cancel = nil
	}
}

// Restart closes existing connections; the supervisor starts a fresh process.
func (s *Server) Restart() error { s.Close(); return nil }
