package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"go.uber.org/zap"
	cloudMedia "kicau.local/edge/internal/cloudmedia/service"
	desktopDelivery "kicau.local/edge/internal/delivery/rest/desktop"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery"
	setupDelivery "kicau.local/edge/internal/delivery/rest/setup"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/network/onvif"
	"kicau.local/edge/internal/infra/storage/credentials"
	"kicau.local/edge/internal/mediaengine/media/ffmpeg"
	"kicau.local/edge/internal/setup/model"
	setupRepo "kicau.local/edge/internal/setup/repository"
	setupUC "kicau.local/edge/internal/setup/usecase"
	setupWorker "kicau.local/edge/internal/setup/worker"
)

// Coordinator keeps setup HTTP alive while replacing an idle media runtime.
// It is the composition-side implementation of the setup Runtime interface.
type Coordinator struct {
	mu            sync.RWMutex
	ops           sync.Mutex
	ctx           context.Context
	base          config.Config
	settings      *setupRepo.Configuration
	setup         *setupDelivery.Handler
	setupFlow     *setupUC.UseCase
	checksJobID   string
	jobs          *setupWorker.Jobs
	initial       model.Configuration
	shell         http.Handler
	current       *engineRuntime
	configuration model.Configuration
	errorCode     string
	closed        bool
}
type initialSettings struct {
	store   *setupRepo.Configuration
	initial model.Configuration
}

func (s initialSettings) Load(ctx context.Context) (model.Configuration, error) {
	c, e := s.store.Load(ctx)
	if errors.Is(e, apperror.ErrNotFound) {
		return s.initial, nil
	}
	return c, e
}
func (s initialSettings) Save(ctx context.Context, c model.Configuration) error {
	return s.store.Save(ctx, c)
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
	m := &Coordinator{ctx: ctx, base: c, settings: setupRepo.New(filepath.Join(c.DataDir, "setup.json"), credentials.Protector{}), jobs: setupWorker.New(ctx)}
	initial := model.Configuration{
		Channels: []model.Channel{},
		NVRIP:    c.NVRIP,
		NVRMAC:   c.NVRMAC,
	}
	networks := onvif.Client{}.Networks()
	if len(networks) > 0 {
		initial.LANIP = networks[0].IP
		initial.LANCIDR = networks[0].CIDR
	}
	for _, ch := range c.Channels {
		initial.Channels = append(initial.Channels, model.Channel{
			ID:     ch.ID,
			IP:     ch.IP,
			MAC:    ch.MAC,
			Source: ch.Source,
		})
	}
	u := setupUC.New(initialSettings{m.settings, initial}, onvif.Client{}, ffmpeg.Inspector{Binary: c.FFmpeg, ProbeBinary: c.FFprobe, MediaBinary: c.MediaMTX}, m, m.jobs)
	m.setup = setupDelivery.New(u, c.LocalControlToken)
	m.setupFlow = u
	m.initial = initial
	bridge, e := cloudMedia.New(filepath.Join(c.DataDir, "cloud-media"))
	if e != nil {
		return nil, e
	}
	m.shell = delivery.Router(c, delivery.Handlers{Setup: m.setup, Desktop: &desktopDelivery.Handler{Cloud: bridge}})
	return m, nil
}

// Restore runs in a background goroutine owned by server, never in a handler or UseCase.
func (m *Coordinator) Restore() {
	m.mu.RLock()
	configured := m.current != nil
	m.mu.RUnlock()
	if configured {
		return
	}
	c, e := m.settings.Load(m.ctx)
	configSource := "setup.json"
	if errors.Is(e, apperror.ErrNotFound) || len(c.Channels) == 0 {
		if len(m.initial.Channels) > 0 {
			c = m.initial
			configSource = ".env"
		} else {
			return
		}
	} else if e != nil {
		m.mu.Lock()
		m.errorCode = "CONFIGURATION_UNAVAILABLE"
		m.mu.Unlock()
		return
	}
	if len(m.initial.Channels) > len(c.Channels) {
		c = m.initial
		configSource = ".env (overriding older setup.json)"
	}
	logger.Info("loaded active configuration",
		zap.String("source", configSource),
		zap.Int("channel_count", len(c.Channels)),
		zap.String("nvr_ip", c.NVRIP),
	)
	for _, ch := range c.Channels {
		logger.Info("channel configured in engine",
			zap.String("id", ch.ID),
			zap.String("ip", ch.IP),
		)
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
	runtime, e := m.launch(c)
	m.mu.Lock()
	if e != nil {
		m.errorCode = "ENGINE_START_FAILED"
		var d *model.Failure
		if errors.As(e, &d) {
			m.errorCode = d.Code
		}
		m.mu.Unlock()
		return
	}
	m.current = runtime
	m.configuration = c
	m.errorCode = ""
	m.mu.Unlock()
	if job, e := m.setupFlow.Start(model.JobRequest{Action: "recheck"}); e == nil {
		m.mu.Lock()
		m.checksJobID = job.ID
		m.mu.Unlock()
	}
}
func (m *Coordinator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Serialize session mutation with reconfiguration, without holding a lock for WebSocket lifetime.
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
func (m *Coordinator) Status(ctx context.Context) model.RuntimeStatus {
	m.mu.RLock()
	r, c, code, checkID := m.current, m.configuration, m.errorCode, m.checksJobID
	m.mu.RUnlock()
	status := model.RuntimeStatus{Configured: r != nil, ErrorCode: code, Targets: []string{}, APIURLs: []string{}, Channels: []model.ViewChannel{}}
	if code != "" {
		status.Advice = model.Advice(code)
	}
	if checkID != "" {
		status.ChecksJobID = checkID
		if job, e := m.jobs.Get(checkID); e == nil && job.State == "SUCCEEDED" {
			status.SourceChecks = job.Result
		}
	}
	if r != nil {
		if len(c.Channels) >= 24 {
			status.CapacityNotice = "Kapasitas 24 channel perlu diuji pada komputer arena; live memakai codec sumber tanpa konversi."
		}
		status.Ready = r.container.Engine.Status().Ready
		if !status.Ready && status.ErrorCode == "" {
			status.ErrorCode = "MEDIA_UNAVAILABLE"
			status.Advice = model.Advice(status.ErrorCode)
		}
		status.Session, _ = r.container.Sessions.Current(ctx)
		status.APIURLs = []string{m.apiOrigin(c.LANIP)}
		for target := range m.base.Destinations {
			status.Targets = append(status.Targets, target)
		}
		sort.Strings(status.Targets)
		for _, v := range c.Channels {
			ch := model.ViewChannel{ID: v.ID, HasAudio: true, VideoCodec: "hevc", LiveVideoCodec: "hevc"}
			if v.Probe != nil {
				ch.HasAudio = v.Probe.HasAudio
				if ch.HasAudio {
					ch.LiveAudioCodec = v.Probe.AudioCodec
				}
				ch.VideoCodec = v.Probe.VideoCodec
				ch.LiveVideoCodec = v.Probe.VideoCodec
				ch.Width = v.Probe.Width
				ch.Height = v.Probe.Height
			}
			status.Channels = append(status.Channels, ch)
		}
	}
	return status
}
func (m *Coordinator) Apply(ctx context.Context, c model.Configuration) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	if m.closed {
		return apperror.ErrUnavailable
	}
	m.mu.RLock()
	old, oldConfiguration := m.current, m.configuration
	m.mu.RUnlock()
	if old != nil && old.container.Sessions.Busy() {
		return apperror.Public(apperror.ErrConflict, "Hentikan sesi dan tunggu finalisasi sebelum mengganti konfigurasi")
	}
	if e := ctx.Err(); e != nil {
		return e
	}
	m.mu.Lock()
	m.current = nil
	m.mu.Unlock()
	if old != nil {
		old.close()
	}
	next, e := m.launch(c)
	if e == nil {
		e = m.settings.Save(ctx, c)
	}
	if e != nil {
		if next != nil {
			next.close()
		}
		logger.Warn("configuration apply failed; restoring previous runtime", zap.Error(e))
		var restored *engineRuntime
		if old != nil {
			restored, _ = m.launch(oldConfiguration)
		}
		m.mu.Lock()
		m.current = restored
		m.configuration = oldConfiguration
		m.errorCode = "CONFIG_APPLY_FAILED"
		m.mu.Unlock()
		return apperror.Public(apperror.ErrUnavailable, "Konfigurasi gagal diterapkan; periksa binary, folder data dan pengaturan jaringan")
	}
	m.mu.Lock()
	m.current = next
	m.checksJobID = ""
	m.configuration = c
	m.errorCode = ""
	m.mu.Unlock()
	return nil
}

func (m *Coordinator) CanConfigure() error {
	m.mu.RLock()
	r := m.current
	m.mu.RUnlock()
	if r != nil && r.container.Sessions.Busy() {
		return &model.Failure{Code: "SESSION_ACTIVE", Message: "Hentikan sesi dan tunggu finalisasi sebelum mengganti konfigurasi"}
	}
	return nil
}
func (m *Coordinator) launch(s model.Configuration) (*engineRuntime, error) {
	networkReady := false
	networks := (onvif.Client{}).Networks()
	for _, n := range networks {
		if s.LANIP == "" || (n.IP == s.LANIP && n.CIDR == s.LANCIDR) {
			networkReady = true
			if s.LANIP == "" {
				s.LANIP = n.IP
				s.LANCIDR = n.CIDR
			}
			break
		}
	}
	if !networkReady && len(networks) > 0 {
		s.LANIP = networks[0].IP
		s.LANCIDR = networks[0].CIDR
		networkReady = true
	}
	if !networkReady {
		s.LANIP = "127.0.0.1"
		s.LANCIDR = "127.0.0.1/32"
		networkReady = true
	}
	c := m.base
	c.LANServerIP = s.LANIP
	c.ReadCIDRs = []string{"127.0.0.1/32", "::1/128", s.LANCIDR}
	c.Channels = []config.Channel{}
	_, port, _ := net.SplitHostPort(c.Listen)
	for _, host := range []string{"localhost", "127.0.0.1", s.LANIP} {
		c.Origins = append(c.Origins, "http://"+net.JoinHostPort(host, port))
	}
	for _, v := range s.Channels {
		audio := true
		if v.Probe != nil {
			audio = v.Probe.HasAudio
		}
		c.Channels = append(c.Channels, config.Channel{ID: v.ID, Source: v.Source, HasAudio: &audio})
	}
	r, e := startRuntime(m.ctx, c)
	if e != nil {
		return nil, e
	}
	r.container.Handlers.Setup = m.setup
	r.handler = delivery.Router(c, r.container.Handlers)
	return r, nil
}

func (m *Coordinator) apiOrigin(ip string) string {
	if m.base.LocalPublicURL != "" {
		return m.base.LocalPublicURL
	}
	_, port, _ := net.SplitHostPort(m.base.Listen)
	return "http://" + net.JoinHostPort(ip, port)
}
func (m *Coordinator) Close() {
	m.jobs.Close()
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
