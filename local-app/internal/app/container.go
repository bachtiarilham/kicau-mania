// Package app assembles application repositories, services, workers, and handlers.
package app

import (
	"context"
	"net"
	"path/filepath"

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
	whepDelivery "kicau.local/edge/internal/delivery/whep"
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
	mediaServer := mediamtx.New(mediamtx.Options{Binary: c.MediaMTX, ConfigFile: c.MediaConfig, RTSPListen: c.RTSPListen, WebRTCListen: c.WebRTCListen, WebRTCLocalUDP: c.WebRTCLocalUDP, WebRTCBase: c.WebRTCBase, ICEHosts: c.ICEHosts, Origins: c.Origins, ReadCIDRs: c.ReadCIDRs, Sources: mediaSources(catalog)})
	return catalog, mediaServer, nil
}
func NewContainer(ctx context.Context, c config.Config) (*Container, error) {
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
	var objectStorage savingUC.Storage
	if c.S3Bucket != "" {
		objectStorage = s3.NewLazy(s3.Options{Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
	}
	var remote audioUC.MetricSender
	if c.BackendURL != "" {
		remote = audioService.NewMetricsClient(c.BackendURL, c.BackendToken)
	}
	channels := channelUC.New(sources, mediaServer)
	audioJobs := audioWorker.New()
	streamingJobs := streamingWorker.New()
	savingJobs := savingWorker.New()
	audio := audioUC.New(audioUC.Options{SampleRate: c.SampleRate, Threshold: c.Threshold}, sources, audioService.NewBroadcaster(), remote, audioJobs)
	streaming := streamingUC.New(sources, ffmpeg.Restreamer{Binary: c.FFmpeg}, c.Destinations, streamingJobs)
	saving := savingUC.New(repo, objectStorage, savingJobs)
	sessionJournal, err := sessionRepo.New(filepath.Join(c.DataDir, "media-sessions"))
	if err != nil {
		return nil, err
	}
	sessions := sessionUC.New(sessionUC.Options{DataDir: c.DataDir, MaxDuration: c.MaxRound, MinFreeBytes: c.MinFreeBytes, Targets: c.Destinations}, sessionJournal, repo, sources, ffmpeg.Pipeline{Binary: c.FFmpeg}, ffmpeg.Finalizer{Binary: c.FFmpeg, Probe: c.FFprobe}, audio, streaming, filesystem.Disk{}, sessionWorker.New(), mediaServer)
	engine := cloudUC.NewEngine(cloudUC.EngineOptions{
		ArenaID: c.ArenaID, EngineID: c.EngineID, PlaybackBaseURL: c.WebRTCBase,
		HeartbeatTimeout: c.HeartbeatTimeout, CommandsConfigured: c.CloudCommandsURL != "",
	}, sources, mediaServer, saving, streaming, journal,
		cloudEndpoint(c.CloudHeartbeatURL, c.CloudToken), cloudEndpoint(c.CloudMediaUpdatesURL, c.CloudToken), cloudEndpoint(c.CloudPlaybackAuthorizeURL, c.CloudToken))
	host, port, _ := net.SplitHostPort(c.WebRTCListen)
	playback, err := whepDelivery.New(engine, "http://"+net.JoinHostPort(host, port), sessions)
	if err != nil {
		return nil, err
	}
	sockets := websocketDelivery.New(audio, c.Origins)
	sessions.BindConnections(playback, sockets)
	var commands cloudUC.CommandCloud
	if c.CloudCommandsURL != "" {
		commands = cloudClient.New(c.CloudCommandsURL, c.CloudToken)
	}
	return &Container{
		Background: cloudWorker.New(), Engine: engine,
		Commands:       cloudUC.NewCommands(journal, commands, cloudUC.NewCommandExecutor(sessions), c.ArenaID, c.EngineID),
		ChannelCatalog: sources, MediaServer: mediaServer, Channels: channels, Audio: audio, Streaming: streaming, Saving: saving, Sessions: sessions,
		Handlers: delivery.Handlers{Engine: engineDelivery.New(engine), WHEP: playback, Channels: channelDelivery.New(channels), WebSocket: sockets, Streaming: streamingDelivery.New(streaming), Saving: savingDelivery.New(saving), Sessions: sessionDelivery.New(sessions)},
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
