package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	cameraModel "kicau.local/edge/internal/camera/model"
	cloudMedia "kicau.local/edge/internal/cloud/service"
	competitionDelivery "kicau.local/edge/internal/competition/delivery"
	competitionUsecase "kicau.local/edge/internal/competition/usecase"
	"kicau.local/edge/internal/delivery"
	desktopDelivery "kicau.local/edge/internal/delivery/rest/desktop"
	setupDelivery "kicau.local/edge/internal/delivery/rest/setup"
	"kicau.local/edge/internal/infra/ffmpeg"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/network"
)

type Coordinator struct {
	mu        sync.RWMutex
	ops       sync.Mutex
	ctx       context.Context
	base      config.Config
	shell     http.Handler
	current   *engineRuntime
	errorCode string
	closed    bool
	setup     *setupDelivery.Handler
	arenaMu   sync.Mutex
	rounds    *competitionDelivery.Handler
}

func NewCoordinator(ctx context.Context, c config.Config) (*Coordinator, error) {
	if c.AccessMode != "lan" || len(c.Token) < 32 {
		return nil, apperror.ErrInvalid
	}
	var secret [32]byte
	if _, e := rand.Read(secret[:]); e != nil {
		return nil, e
	}
	c.LocalControlToken = hex.EncodeToString(secret[:])
	m := &Coordinator{ctx: ctx, base: c}
	m.setup = setupDelivery.New(m, c.LocalControlToken)
	m.rounds = competitionDelivery.New(competitionUsecase.NewRounds(time.Now))
	bridge, e := cloudMedia.New(filepath.Join(c.DataDir, "cloud-media"))
	if e != nil {
		return nil, e
	}
	m.shell = delivery.Router(c, delivery.Handlers{Setup: m.setup, Desktop: &desktopDelivery.Handler{Cloud: bridge}, Rounds: m.rounds})
	return m, nil
}

func (m *Coordinator) Restore() {
	m.mu.RLock()
	configured := m.current != nil
	m.mu.RUnlock()
	if configured {
		return
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	if m.closed {
		return
	}
	m.mu.RLock()
	alreadyConfigured := m.current != nil
	m.mu.RUnlock()
	if alreadyConfigured {
		return
	}
	runtime, e := m.launch()
	m.mu.Lock()
	if e != nil {
		m.errorCode = "ENGINE_START_FAILED"
		var d *cameraModel.Failure
		if errors.As(e, &d) {
			m.errorCode = d.Code
		}
		m.mu.Unlock()
		return
	}
	m.current = runtime
	m.errorCode = ""
	m.mu.Unlock()
}

func (m *Coordinator) launch() (*engineRuntime, error) {
	c := m.base
	r, e := startRuntime(m.ctx, c)
	if e != nil {
		return nil, e
	}
	r.container.Handlers.Setup = m.setup
	r.container.Handlers.Rounds = m.rounds
	r.handler = delivery.Router(c, r.container.Handlers)
	return r, nil
}

func (m *Coordinator) Networks() []network.Network {
	return network.Interfaces()
}

func (m *Coordinator) Check(ctx context.Context) []setupDelivery.Diagnostic {
	v := (ffmpeg.Inspector{Binary: m.base.FFmpeg, ProbeBinary: m.base.FFprobe, MediaBinary: m.base.MediaMTX}).Check(ctx)
	out := make([]setupDelivery.Diagnostic, len(v))
	for i, d := range v {
		out[i] = setupDelivery.Diagnostic{
			Component: d.Component,
			Code:      d.Code,
			Message:   d.Message,
			Advice:    setupDelivery.Advice(d.Code),
		}
	}
	return out
}

func (m *Coordinator) Configuration(ctx context.Context) (setupDelivery.Configuration, error) {
	channels := make([]setupDelivery.ViewChannel, len(m.base.Channels))
	for i, ch := range m.base.Channels {
		masked, hasCreds := setupDelivery.MaskSource(ch.Source)
		hasAudio := true
		if ch.HasAudio != nil {
			hasAudio = *ch.HasAudio
		}
		channels[i] = setupDelivery.ViewChannel{
			ID:             ch.ID,
			IP:             ch.IP,
			Source:         masked,
			HasAudio:       hasAudio,
			HasCredentials: hasCreds,
		}
	}
	lanIP := m.base.LANServerIP
	if lanIP == "" {
		nets := network.Interfaces()
		if len(nets) > 0 {
			lanIP = nets[0].IP
		}
	}
	return setupDelivery.Configuration{
		Version:  1,
		LANIP:    lanIP,
		NVRIP:    m.base.NVRIP,
		NVRMAC:   m.base.NVRMAC,
		Channels: channels,
	}, nil
}

func (m *Coordinator) Status(ctx context.Context) setupDelivery.RuntimeStatus {
	m.mu.RLock()
	r, code := m.current, m.errorCode
	m.mu.RUnlock()

	channels := make([]setupDelivery.ViewChannel, len(m.base.Channels))
	for i, ch := range m.base.Channels {
		masked, hasCreds := setupDelivery.MaskSource(ch.Source)
		hasAudio := true
		if ch.HasAudio != nil {
			hasAudio = *ch.HasAudio
		}
		channels[i] = setupDelivery.ViewChannel{
			ID:             ch.ID,
			IP:             ch.IP,
			Source:         masked,
			HasAudio:       hasAudio,
			HasCredentials: hasCreds,
		}
	}

	status := setupDelivery.RuntimeStatus{
		Configured: r != nil,
		ErrorCode:  code,
		Advice:     setupDelivery.Advice(code),
		Channels:   channels,
		Targets:    []string{},
		APIURLs:    []string{m.apiOrigin(m.base.LANServerIP)},
	}
	for target := range m.base.Destinations {
		status.Targets = append(status.Targets, target)
	}
	sort.Strings(status.Targets)
	if r != nil {
		status.Ready = r.container.Engine.Status().Ready
		if !status.Ready && status.ErrorCode == "" {
			status.ErrorCode = "MEDIA_UNAVAILABLE"
			status.Advice = setupDelivery.Advice(status.ErrorCode)
		}
		status.Session, _ = r.container.Sessions.Current(ctx)
	}
	return status
}

func (m *Coordinator) Guidance(code string) setupDelivery.Diagnostic {
	return setupDelivery.Diagnostic{Code: code, Advice: setupDelivery.Advice(code)}
}

func (m *Coordinator) Start(in setupDelivery.JobRequest) (setupDelivery.Job, error) {
	return setupDelivery.Job{ID: "done", State: "SUCCEEDED", Progress: "Complete"}, nil
}

func (m *Coordinator) Job(id string) (setupDelivery.Job, error) {
	return setupDelivery.Job{ID: id, State: "SUCCEEDED", Progress: "Complete"}, nil
}

func (m *Coordinator) Cancel(id string) error {
	return nil
}

func (m *Coordinator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" && !strings.HasPrefix(r.URL.Path, "/api/v1/setup/") {
		m.ops.Lock()
		defer m.ops.Unlock()
	}
	m.mu.RLock()
	runtime := m.current
	handler := m.shell
	if runtime != nil {
		handler = runtime.handler
	}
	m.mu.RUnlock()
	handler.ServeHTTP(w, r)
}

func (m *Coordinator) apiOrigin(ip string) string {
	if m.base.LocalPublicURL != "" {
		return m.base.LocalPublicURL
	}
	_, port, _ := net.SplitHostPort(m.base.Listen)
	return "http://" + net.JoinHostPort(ip, port)
}

func (m *Coordinator) Close() {
	m.ops.Lock()
	defer m.ops.Unlock()
	m.closed = true
	m.mu.Lock()
	r := m.current
	m.current = nil
	m.mu.Unlock()
	if r != nil {
		r.close()
	}
}

type engineRuntime struct {
	container *Container
	handler   http.Handler
	cancel    context.CancelFunc
	mediaDone chan struct{}
	started   []interface{ Close() }
}

func startRuntime(parent context.Context, c config.Config) (*engineRuntime, error) {
	ctx, cancel := context.WithCancel(parent)
	container, e := NewContainer(ctx, c)
	if e != nil {
		cancel()
		return nil, e
	}
	r := &engineRuntime{container: container, cancel: cancel, mediaDone: make(chan struct{})}
	go func() {
		defer close(r.mediaDone)
		defer container.MediaServer.Close()
		for ctx.Err() == nil {
			if err := container.MediaServer.Start(ctx); err == nil {
				select {
				case <-ctx.Done():
				case <-container.MediaServer.Done():
				}
				if ctx.Err() == nil {
					logger.Warn("MediaMTX exited; check pinned version and listen ports")
				}
				container.MediaServer.Close()
			} else if ctx.Err() == nil {
				logger.Warn("MediaMTX unavailable; setup API remains available", zap.Error(err))
			}
			t := time.NewTimer(5 * time.Second)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}()
	for _, module := range []interface {
		Start() error
		Close()
	}{container.Saving, container.Audio, container.Streaming} {
		if e = module.Start(); e != nil {
			r.close()
			return nil, e
		}
		r.started = append(r.started, module)
	}
	if e = container.Sessions.Start(); e != nil {
		r.close()
		return nil, e
	}
	if e = container.Background.Start(); e != nil {
		r.close()
		return nil, e
	}
	if c.CloudCommandsURL != "" {
		container.Background.Loop("cloud-commands", 5*time.Second, container.Commands.Poll)
	}
	if c.CloudHeartbeatURL != "" {
		container.Background.Loop("heartbeat", c.HeartbeatInterval, container.Engine.Heartbeat)
	}
	if c.CloudMediaUpdatesURL != "" {
		container.Background.Loop("media-metadata", 5*time.Second, container.Engine.PublishMetadata)
	}
	return r, nil
}

func (r *engineRuntime) close() {
	r.container.Background.Close()
	r.container.Sessions.Close()
	r.container.Handlers.Streaming.Close()
	for i := len(r.started) - 1; i >= 0; i-- {
		r.started[i].Close()
	}
	r.cancel()
	<-r.mediaDone
}
