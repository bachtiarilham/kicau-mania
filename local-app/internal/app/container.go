// Package app assembles application repositories, services, workers, and handlers.
package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	cloudMedia "kicau.local/edge/internal/cloud/service"
	desktopDelivery "kicau.local/edge/internal/delivery/rest/desktop"
	"kicau.local/edge/internal/infra/logger"

	"kicau.local/edge/config"
	cameraDelivery "kicau.local/edge/internal/camera/delivery"
	cameraModel "kicau.local/edge/internal/camera/model"
	cameraRepo "kicau.local/edge/internal/camera/repository"
	cameraService "kicau.local/edge/internal/camera/service"
	cameraUC "kicau.local/edge/internal/camera/usecase"
	cloudClient "kicau.local/edge/internal/cloud/client"
	cloudRepo "kicau.local/edge/internal/cloud/repository"
	cloudUC "kicau.local/edge/internal/cloud/usecase"
	cloudWorker "kicau.local/edge/internal/cloud/worker"
	delivery "kicau.local/edge/internal/delivery"
	engineDelivery "kicau.local/edge/internal/delivery/rest/engine"
	"kicau.local/edge/internal/infra/ffmpeg"
	"kicau.local/edge/internal/infra/mediamtx"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"kicau.local/edge/internal/infra/storage/s3"
	recordingDelivery "kicau.local/edge/internal/recording/delivery"
	recordingRepo "kicau.local/edge/internal/recording/repository"
	recordingUC "kicau.local/edge/internal/recording/usecase"
	recordingWorker "kicau.local/edge/internal/recording/worker"
	streamingDelivery "kicau.local/edge/internal/streaming/delivery"
	streamingService "kicau.local/edge/internal/streaming/service"
	streamingUC "kicau.local/edge/internal/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/streaming/worker"
)

type Container struct {
	Background     *cloudWorker.Worker
	Engine         *cloudUC.Engine
	Commands       *cloudUC.Commands
	ChannelCatalog *cameraRepo.Catalog
	MediaServer    *mediamtx.Server
	Channels       *cameraUC.CameraUseCase
	Audio          *streamingUC.AudioUseCase
	Streaming      *streamingUC.RestreamUseCase
	Saving         *recordingUC.SavingUseCase
	Sessions       *recordingUC.SessionUseCase
	Zoom           *cameraUC.ZoomUseCase
	Handlers       delivery.Handlers
}

// NewMedia is also used by -render-config: rendering does not start processes.
func NewMedia(c config.Config) (*cameraRepo.Catalog, *mediamtx.Server, error) {
	inputs := make([]cameraRepo.Input, 0, len(c.Channels))
	for _, ch := range c.Channels {
		inputs = append(inputs, cameraRepo.Input{ID: ch.ID, Source: ch.Source})
	}
	catalog, e := cameraRepo.New(inputs, c.RTSPBase)
	if e != nil {
		return nil, nil, e
	}
	mediaSources, err := mediaSources(c, catalog)
	if err != nil {
		return nil, nil, err
	}
	mediaServer := mediamtx.New(mediamtx.Options{Binary: c.MediaMTX, ConfigFile: c.MediaConfig, RTSPListen: c.RTSPListen, JudgeReadCIDRs: c.JudgeReadCIDRs, OperatorCamera: config.ArenaCameraID, Sources: mediaSources})
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
				logger.Info("camera online / probe success",
					zap.String("channel_id", c.Channels[idx].ID),
					zap.String("ip", c.Channels[idx].IP),
					zap.String("video_codec", p.VideoCodec),
					zap.String("audio_codec", p.AudioCodec),
					zap.Bool("has_audio", p.HasAudio))
			} else {
				var failure *cameraModel.Failure
				if errors.As(err, &failure) && failure.Code == "UNSUPPORTED_VIDEO_CODEC" {
					errCh <- err
					return
				}
				logger.Warn("camera offline / probe unreachable",
					zap.String("channel_id", c.Channels[idx].ID),
					zap.String("ip", c.Channels[idx].IP),
					zap.Error(err))
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
	repo, err := recordingRepo.NewFiles(filepath.Join(c.DataDir, "queue"))
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
	var objectStorage recordingUC.Storage
	if c.AccessMode == "lan" {
		objectStorage = bridge
	}
	if c.S3Bucket != "" && c.AccessMode != "lan" {
		objectStorage = s3.NewLazy(s3.Options{Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
	}
	var remote streamingUC.MetricSender
	if c.BackendURL != "" {
		remote = streamingService.NewMetricsClient(c.BackendURL, c.BackendToken)
	}
	channels := cameraUC.NewCameraUseCase(sources, sources)
	audioJobs := streamingWorker.NewAudioWorker()
	streamingJobs := streamingWorker.NewRestreamWorker()
	savingJobs := recordingWorker.NewUploadWorker()
	previewSources := make(map[string]string, len(c.Channels))
	for _, ch := range c.Channels {
		if ch.HasAudio == nil || *ch.HasAudio {
			previewSources[ch.ID], _ = sources.HubRTSP(ch.ID)
		}
	}
	pipeline := ffmpeg.Pipeline{Binary: c.FFmpeg, Probe: c.FFprobe, AudioFilter: audioFilterOptions(c)}
	audio := streamingUC.NewAudioUseCase(streamingUC.AudioOptions{
		SampleRate: c.SampleRate, Threshold: c.Threshold,
		PreviewSources: previewSources, PreviewCapture: pipeline.RunPreviewAudio,
	}, sources, streamingService.NewBroadcaster(), remote, audioJobs)
	streaming := streamingUC.NewRestreamUseCase(sources, ffmpeg.Restreamer{Binary: c.FFmpeg, Probe: c.FFprobe}, c.Destinations, streamingJobs)
	saving := recordingUC.NewSavingUseCase(repo, objectStorage, savingJobs)
	sessionJournal, err := recordingRepo.NewJournal(filepath.Join(c.DataDir, "media-sessions"))
	if err != nil {
		return nil, err
	}
	sessions := recordingUC.NewSessionUseCase(recordingUC.SessionOptions{DataDir: c.DataDir, RecordingsDir: c.RecordingsDir, MaxDuration: c.MaxRound, MinFreeBytes: c.MinFreeBytes, Targets: c.Destinations}, sessionJournal, repo, sources, pipeline, ffmpeg.Finalizer{Binary: c.FFmpeg, Probe: c.FFprobe}, audio, streaming, filesystem.Disk{}, recordingWorker.NewSessionWorker(), mediaServer)
	engine := cloudUC.NewEngine(cloudUC.EngineOptions{
		ArenaID: c.ArenaID, EngineID: c.EngineID,
		HeartbeatTimeout: c.HeartbeatTimeout, CommandsConfigured: c.CloudCommandsURL != "",
		LAN: c.AccessMode == "lan",
	}, sources, mediaServer, saving, streaming, journal,
		cloudEndpoint(c.CloudHeartbeatURL, c.CloudToken), cloudEndpoint(c.CloudMediaUpdatesURL, c.CloudToken))

	streamingHandler := streamingDelivery.NewHandler(streaming, audio, c.Origins)
	sessions.BindConnections(nil, streamingHandler)
	var commands cloudUC.CommandCloud
	if c.CloudCommandsURL != "" {
		commands = cloudClient.New(c.CloudCommandsURL, c.CloudToken)
	}

	zoomRepo := cameraRepo.NewFileRepository(filepath.Join(c.DataDir, "channels_zoom.json"))
	zoomBuilder := cameraService.NewBuilder(c.FFmpeg)
	zoomUC, err := cameraUC.NewZoomUseCase(zoomRepo, zoomBuilder)
	if err != nil {
		return nil, err
	}
	cameraHandler := cameraDelivery.NewHandler(channels, zoomUC)
	recordingHandler := recordingDelivery.NewHandler(sessions, saving)

	return &Container{
		Background: cloudWorker.New(), Engine: engine,
		Commands:       cloudUC.NewCommands(journal, commands, cloudUC.NewCommandExecutor(sessions), c.ArenaID, c.EngineID),
		ChannelCatalog: sources, MediaServer: mediaServer, Channels: channels, Audio: audio, Streaming: streaming, Saving: saving, Sessions: sessions,
		Zoom: zoomUC,
		Handlers: delivery.Handlers{
			Desktop:   &desktopDelivery.Handler{Cloud: bridge, Saving: saving},
			Engine:    engineDelivery.New(engine),
			Camera:    cameraHandler,
			Recording: recordingHandler,
			Streaming: streamingHandler,
		},
	}, nil
}

func cloudEndpoint(endpoint, token string) cloudUC.Poster {
	if endpoint == "" {
		return nil
	}
	return cloudClient.New(endpoint, token)
}

func audioFilterOptions(c config.Config) ffmpeg.AudioFilterOptions {
	return ffmpeg.AudioFilterOptions{
		Enabled: c.AudioFilterEnabled, HighpassFreq: c.AudioHighpassFreq,
		GateThreshold: c.AudioGateThreshold, GateRatio: c.AudioGateRatio,
		GateAttack: c.AudioGateAttack, GateRelease: c.AudioGateRelease,
		ApplyRecording: c.AudioFilterApplyRecording,
	}
}

func mediaSources(c config.Config, catalog *cameraRepo.Catalog) ([]mediamtx.Source, error) {
	mapping, err := cameraRepo.NewFileRepository(filepath.Join(c.DataDir, "channels_zoom.json")).Load()
	if err != nil {
		return nil, err
	}
	builder := cameraService.NewBuilder(c.FFmpeg)
	builder.AudioFilter = (ffmpeg.Pipeline{AudioFilter: audioFilterOptions(c)}).AudioFilterString()
	base := strings.TrimRight(c.RTSPBase, "/")

	inputs := catalog.Inputs()
	sources := make([]mediamtx.Source, 0, len(inputs))
	for _, in := range inputs {
		sources = append(sources, mediamtx.Source{ID: in.ID, URL: in.Source})
	}
	for _, in := range inputs {
		source, _ := catalog.HubRTSP(in.ID)
		id := "zoom/" + in.ID + "_zoom"
		sources = append(sources, mediamtx.Source{ID: id,
			OnDemand: append([]string{builder.FFmpegBinary}, builder.BuildArgs(cameraModel.ZoomChannel{}, source, base+"/"+id)...),
		})
	}
	for _, ch := range mapping.ActiveChannels {
		source, err := catalog.HubRTSP(ch.SourceCamera)
		if !ch.Active || err != nil || ch.Validate() != nil {
			continue
		}
		id := ch.ChannelID + "_zoom"
		if _, err := catalog.HubRTSP(id); err == nil {
			continue
		}
		sources = append(sources, mediamtx.Source{ID: id,
			OnDemand: append([]string{builder.FFmpegBinary}, builder.BuildArgs(ch, source, base+"/"+id)...),
		})
	}
	return sources, nil
}
