// Package mediamtx configures and manages the local MediaMTX server.
package mediamtx

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"go.uber.org/zap"

	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/runtime/subprocess"
	"kicau.local/edge/internal/mediaengine/media"
)

var credsRegex = regexp.MustCompile(`://([^:]+):([^@]+)@`)

type mediaMTXLogger struct{}

func (m *mediaMTXLogger) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sanitized := credsRegex.ReplaceAllString(line, "://$1:***@")
		if strings.Contains(line, "ERR") || strings.Contains(line, "WAR") {
			logger.Warn("mediamtx", zap.String("log", sanitized))
		} else {
			logger.Info("mediamtx", zap.String("log", sanitized))
		}
	}
	return len(p), nil
}

type Source struct {
	ID, URL  string
	OnDemand []string
}

type Options struct {
	Sources                        []Source
	Binary, ConfigFile, RTSPListen string
	ReadCIDRs                      []string
}
type Server struct {
	opts   Options
	mu     sync.Mutex
	p      media.Process
	cancel context.CancelFunc
	launch func(context.Context, string, []string) (media.Process, error)
}

func New(opts Options) *Server {
	opts.Sources = append([]Source(nil), opts.Sources...)
	for i := range opts.Sources {
		opts.Sources[i].OnDemand = append([]string(nil), opts.Sources[i].OnDemand...)
	}
	logWriter := &mediaMTXLogger{}
	return &Server{opts: opts, launch: func(ctx context.Context, binary string, args []string) (media.Process, error) {
		return subprocess.StartMediaServer(ctx, binary, args, logWriter)
	}}
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
	permissions := []any{map[string]any{"action": "read", "path": ""}}
	for _, in := range s.opts.Sources {
		if len(in.OnDemand) > 0 {
			command := make([]string, len(in.OnDemand))
			for i, arg := range in.OnDemand {
				command[i] = strconv.Quote(arg)
			}
			paths[in.ID] = map[string]any{
				"source": "publisher", "overridePublisher": false,
				"runOnDemand":             strings.Join(command, " "),
				"runOnDemandStartTimeout": "20s",
			}
			permissions = append(permissions, map[string]any{"action": "publish", "path": in.ID})
			continue
		}
		if in.URL == "" {
			paths[in.ID] = map[string]any{"source": "publisher", "overridePublisher": false}
			continue
		}
		paths[in.ID] = map[string]any{
			"source":                   in.URL,
			"rtspTransport":            "tcp",
			"sourceOnDemand":           true,
			"sourceOnDemandCloseAfter": "10s",
		}
	}
	authUsers := []any{
		map[string]any{
			"user":        "any",
			"ips":         []string{"127.0.0.1/32", "::1/128"},
			"permissions": permissions},
	}
	cfg := map[string]any{
		"logLevel":          "info",
		"rtsp":              true,
		"rtspAddress":       s.opts.RTSPListen,
		"rtspTransports":    []string{"tcp"},
		"rtmp":              false,
		"hls":               false,
		"srt":               false,
		"api":               false,
		"playback":          false,
		"webrtc":            false,
		"authMethod":        "internal",
		"authInternalUsers": authUsers,
		"paths":             paths,
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
