// Package app assembles application repositories, services, workers, and handlers.
package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	cloudMedia "kicau.local/edge/internal/cloudmedia/service"
	desktopDelivery "kicau.local/edge/internal/delivery/rest/desktop"
	"kicau.local/edge/internal/infra/logger"
	setup "kicau.local/edge/internal/setup/model"

	"kicau.local/edge/config"
	cloudClient "kicau.local/edge/internal/cloud/client"
	cloudRepo "kicau.local/edge/internal/cloud/repository"
	cloudUC "kicau.local/edge/internal/cloud/usecase"
	cloudWorker "kicau.local/edge/internal/cloud/worker"
	delivery "kicau.local/edge/internal/delivery"
	channelDelivery "kicau.local/edge/internal/delivery/rest/channel"
	engineDelivery "kicau.local/edge/internal/delivery/rest/engine"
	savingDelivery "kicau.local/edge/internal/delivery/rest/saving"
	sessionDelivery "kicau.local/edge/internal/delivery/rest/session"
	streamingDelivery "kicau.local/edge/internal/delivery/rest/streaming"
	websocketDelivery "kicau.local/edge/internal/delivery/websocket"
	zoomDelivery "kicau.local/edge/internal/delivery/rest/zoom"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"kicau.local/edge/internal/infra/storage/s3"
	audioService "kicau.local/edge/internal/mediaengine/audio/service"
	audioUC "kicau.local/edge/internal/mediaengine/audio/usecase"
	audioWorker "kicau.local/edge/internal/mediaengine/audio/worker"
	channelRepo "kicau.local/edge/internal/mediaengine/channel/repository"
	channelUC "kicau.local/edge/internal/mediaengine/channel/usecase"
	"kicau.local/edge/internal/mediaengine/media/ffmpeg"
	"kicau.local/edge/internal/mediaengine/media/mediamtx"
	savingRepo "kicau.local/edge/internal/mediaengine/saving/repository"
	savingUC "kicau.local/edge/internal/mediaengine/saving/usecase"
	savingWorker "kicau.local/edge/internal/mediaengine/saving/worker"
	sessionRepo "kicau.local/edge/internal/mediaengine/session/repository"
	sessionUC "kicau.local/edge/internal/mediaengine/session/usecase"
	sessionWorker "kicau.local/edge/internal/mediaengine/session/worker"
	streamingUC "kicau.local/edge/internal/mediaengine/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/mediaengine/streaming/worker"
	zoomRepository "kicau.local/edge/internal/mediaengine/zoom/repository"
	zoomService "kicau.local/edge/internal/mediaengine/zoom/service"
	zoomUsecase "kicau.local/edge/internal/mediaengine/zoom/usecase"
)

type Container struct {
	Background     *cloudWorker.Worker
	Engine         *cloudUC.Engine
	Commands       *cloudUC.Commands
	ChannelCatalog *channelRepo.Catalog
	MediaServer    *mediamtx.Server
	Channels       *channelUC.UseCase
	Audio          *audioUC.UseCase
	Streaming      *streamingUC.UseCase
	Saving         *savingUC.UseCase
	Sessions       *sessionUC.UseCase
	Zoom           *zoomUsecase.UseCase
	Handlers       delivery.Handlers
}

// NewMedia is also used by -render-config: rendering does not start processes.
func NewMedia(c config.Config) (*channelRepo.Catalog, *mediamtx.Server, error) {
	inputs := make([]channelRepo.Input, 0, len(c.Channels))
	for _, ch := range c.Channels {
		inputs = append(inputs, channelRepo.Input{ID: ch.ID, Source: ch.Source})
	}
	catalog, e := channelRepo.New(inputs, c.RTSPBase)
	if e != nil {
		return nil, nil, e
	}
	mediaServer := mediamtx.New(mediamtx.Options{Binary: c.MediaMTX, ConfigFile: c.MediaConfig, RTSPListen: c.RTSPListen, ReadCIDRs: c.ReadCIDRs, Sources: mediaSources(catalog)})
	return catalog, mediaServer, nil
}
func NewContainer(ctx context.Context, c config.Config) (*Container, error) {
	var wg sync.WaitGroup
	errCh := make(chan error, len(c.Channels))
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	for i := range c.Channels {
		if c.Channels[i].HasAudio == nil {
			hasAudio := true
			c.Channels[i].HasAudio = &hasAudio
		}
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p, err := (ffmpeg.Inspector{ProbeBinary: c.FFprobe}).Probe(probeCtx, c.Channels[idx].Source)
			if err == nil {
				hasAudio := p.HasAudio
				c.Channels[idx].HasAudio = &hasAudio
			} else {
				var failure *setup.Failure
				if errors.As(err, &failure) && failure.Code == "UNSUPPORTED_VIDEO_CODEC" {
					errCh <- err
					return
				}
				logger.Warn("channel probe unreached during startup, continuing with fallback", zap.String("channel_id", c.Channels[idx].ID), zap.Error(err))
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	if err, ok := <-errCh; ok {
		return nil, err
	}
	sources, mediaServer, e := NewMedia(c)
	if e != nil {
		return nil, e
	}
	repo, err := savingRepo.NewFiles(filepath.Join(c.DataDir, "queue"))
	if err != nil {
		return nil, err
	}
	journal, err := cloudRepo.New(c.DataDir)
	if err != nil {
		return nil, err
	}
	bridge, err := cloudMedia.New(filepath.Join(c.DataDir, "cloud-media"))
	if err != nil {
		return nil, err
	}
	var objectStorage savingUC.Storage
	if c.AccessMode == "lan" {
		objectStorage = bridge
	}
	if c.S3Bucket != "" && c.AccessMode != "lan" {
		objectStorage = s3.NewLazy(s3.Options{Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
	}
	var remote audioUC.MetricSender
	if c.BackendURL != "" {
		remote = audioService.NewMetricsClient(c.BackendURL, c.BackendToken)
	}
	channels := channelUC.New(sources, sources)
	audioJobs := audioWorker.New()
	streamingJobs := streamingWorker.New()
	savingJobs := savingWorker.New()
	audio := audioUC.New(audioUC.Options{SampleRate: c.SampleRate, Threshold: c.Threshold}, sources, audioService.NewBroadcaster(), remote, audioJobs)
	streaming := streamingUC.New(sources, ffmpeg.Restreamer{Binary: c.FFmpeg, Probe: c.FFprobe}, c.Destinations, streamingJobs)
	saving := savingUC.New(repo, objectStorage, savingJobs)
	sessionJournal, err := sessionRepo.New(filepath.Join(c.DataDir, "media-sessions"))
	if err != nil {
		return nil, err
	}
	pipeline := ffmpeg.Pipeline{
		Binary: c.FFmpeg,
		Probe:  c.FFprobe,
		AudioFilter: ffmpeg.AudioFilterOptions{
			Enabled:        c.AudioFilterEnabled,
			HighpassFreq:   c.AudioHighpassFreq,
			GateThreshold:  c.Threshold,
			GateRatio:      c.AudioGateRatio,
			GateAttack:     c.AudioGateAttack,
			GateRelease:    c.AudioGateRelease,
			ApplyRecording: c.AudioFilterApplyRecording,
		},
	}
	sessions := sessionUC.New(sessionUC.Options{DataDir: c.DataDir, MaxDuration: c.MaxRound, MinFreeBytes: c.MinFreeBytes, Targets: c.Destinations}, sessionJournal, repo, sources, pipeline, ffmpeg.Finalizer{Binary: c.FFmpeg, Probe: c.FFprobe}, audio, streaming, filesystem.Disk{}, sessionWorker.New(), mediaServer)
	engine := cloudUC.NewEngine(cloudUC.EngineOptions{
		ArenaID: c.ArenaID, EngineID: c.EngineID,
		HeartbeatTimeout: c.HeartbeatTimeout, CommandsConfigured: c.CloudCommandsURL != "",
		LAN: c.AccessMode == "lan",
	}, sources, mediaServer, saving, streaming, journal,
		cloudEndpoint(c.CloudHeartbeatURL, c.CloudToken), cloudEndpoint(c.CloudMediaUpdatesURL, c.CloudToken))
	sockets := websocketDelivery.New(audio, c.Origins)
	sessions.BindConnections(nil, sockets)
	var commands cloudUC.CommandCloud
	if c.CloudCommandsURL != "" {
		commands = cloudClient.New(c.CloudCommandsURL, c.CloudToken)
	}

	zoomRepo := zoomRepository.NewFileRepository(filepath.Join(c.DataDir, "channels_zoom.json"))
	zoomBuilder := zoomService.NewBuilder(c.FFmpeg)
	zoomUC, err := zoomUsecase.New(zoomRepo, zoomBuilder)
	if err != nil {
		return nil, err
	}
	zoomHandler := zoomDelivery.New(zoomUC)

	return &Container{
		Background: cloudWorker.New(), Engine: engine,
		Commands:       cloudUC.NewCommands(journal, commands, cloudUC.NewCommandExecutor(sessions), c.ArenaID, c.EngineID),
		ChannelCatalog: sources, MediaServer: mediaServer, Channels: channels, Audio: audio, Streaming: streaming, Saving: saving, Sessions: sessions,
		Zoom:           zoomUC,
		Handlers: delivery.Handlers{
			Desktop:   &desktopDelivery.Handler{Cloud: bridge, Saving: saving},
			Engine:    engineDelivery.New(engine),
			Channels:  channelDelivery.New(channels),
			WebSocket: sockets,
			Streaming: streamingDelivery.New(streaming),
			Saving:    savingDelivery.New(saving),
			Sessions:  sessionDelivery.New(sessions),
			Zoom:      zoomHandler,
		},
	}, nil
}

func cloudEndpoint(endpoint, token string) cloudUC.Poster {
	if endpoint == "" {
		return nil
	}
	return cloudClient.New(endpoint, token)
}

func mediaSources(catalog *channelRepo.Catalog) []mediamtx.Source {
	inputs := catalog.Inputs()
	sources := make([]mediamtx.Source, 0, len(inputs))
	for _, in := range inputs {
		sources = append(sources, mediamtx.Source{ID: in.ID, URL: in.Source})
	}
	return sources
}
