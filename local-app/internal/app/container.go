package app

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	arenaDelivery "kicau.local/edge/internal/core/arena/delivery"
	arenaRepo "kicau.local/edge/internal/core/arena/repository"
	arenaUC "kicau.local/edge/internal/core/arena/usecase"
	arenaWorker "kicau.local/edge/internal/core/arena/worker"
	delivery "kicau.local/edge/internal/delivery/http"
	operationUC "kicau.local/edge/internal/edgetocloud/operation/usecase"
	syncClient "kicau.local/edge/internal/edgetocloud/synchronization/client"
	syncUC "kicau.local/edge/internal/edgetocloud/synchronization/usecase"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"kicau.local/edge/internal/infra/storage/s3"
	audioDelivery "kicau.local/edge/internal/mediaengine/audio/delivery"
	"kicau.local/edge/internal/mediaengine/audio/metricsclient"
	"kicau.local/edge/internal/mediaengine/audio/pubsub"
	audioUC "kicau.local/edge/internal/mediaengine/audio/usecase"
	audioWorker "kicau.local/edge/internal/mediaengine/audio/worker"
	channelRepo "kicau.local/edge/internal/mediaengine/channel/repository"
	"kicau.local/edge/internal/mediaengine/media/ffmpeg"
	"kicau.local/edge/internal/mediaengine/media/mediamtx"
	savingDelivery "kicau.local/edge/internal/mediaengine/saving/delivery"
	savingModel "kicau.local/edge/internal/mediaengine/saving/model"
	savingRepo "kicau.local/edge/internal/mediaengine/saving/repository"
	savingUC "kicau.local/edge/internal/mediaengine/saving/usecase"
	savingWorker "kicau.local/edge/internal/mediaengine/saving/worker"
	streamingDelivery "kicau.local/edge/internal/mediaengine/streaming/delivery"
	streamingUC "kicau.local/edge/internal/mediaengine/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/mediaengine/streaming/worker"
	videoDelivery "kicau.local/edge/internal/mediaengine/video/delivery"
	videoUC "kicau.local/edge/internal/mediaengine/video/usecase"
)

type Container struct {
	ArenaStore      *arenaRepo.SQLite
	Arena           *arenaUC.UseCase
	Background      *arenaWorker.Worker
	Synchronization *syncUC.UseCase
	Commands        *syncUC.Commands
	ChannelCatalog  *channelRepo.Catalog
	MediaServer     *mediamtx.Server
	Video           *videoUC.UseCase
	Audio           *audioUC.UseCase
	Streaming       *streamingUC.UseCase
	Saving          *savingUC.UseCase
	Handlers        delivery.Handlers
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
	recordingFiles, recordingError := savingRepo.NewFiles(filepath.Join(c.DataDir, "queue"))
	var repo savingUC.RecordingRepository = recordingFiles
	if recordingError != nil {
		slog.Warn("recording repository unavailable; judging remains available")
		repo = unavailableRecordings{}
	}
	var objectStorage savingUC.Storage
	if c.S3Bucket != "" {
		objectStorage = s3.NewLazy(s3.Options{Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
	}
	var remote audioUC.MetricSender
	if c.BackendURL != "" {
		remote = metricsclient.New(c.BackendURL, c.BackendToken)
	}
	video := videoUC.New(sources, mediaServer)
	audioJobs := audioWorker.New()
	streamingJobs := streamingWorker.New()
	savingJobs := savingWorker.New()
	audio := audioUC.New(audioUC.Options{SampleRate: c.SampleRate, Threshold: c.Threshold}, sources, ffmpeg.AudioDecoder{Binary: c.FFmpeg}, pubsub.New(), remote, audioJobs)
	streaming := streamingUC.New(sources, ffmpeg.Restreamer{Binary: c.FFmpeg}, c.Destinations, streamingJobs)
	saving := savingUC.New(savingUC.Options{DataDir: c.DataDir, MaxRound: c.MaxRound, MinFreeBytes: c.MinFreeBytes}, sources, repo, ffmpeg.Recorder{Binary: c.FFmpeg}, objectStorage, filesystem.Disk{}, savingJobs)
	store, err := arenaRepo.Open(filepath.Join(c.DataDir, "arena.sqlite"))
	if err != nil {
		audioJobs.Close()
		streamingJobs.Close()
		savingJobs.Close()
		return nil, err
	}
	var packages arenaUC.PackageSource
	if c.CloudPackagesURL != "" {
		packages = syncClient.New(c.CloudPackagesURL, c.CloudToken)
	}
	arena := arenaUC.New(store, mediaServer, arenaUC.Options{ArenaID: c.ArenaID, EngineID: c.EngineID, BackupDir: c.BackupDir, PackageSource: packages})
	if err := arena.Recover(ctx); err != nil {
		store.Close()
		audioJobs.Close()
		streamingJobs.Close()
		savingJobs.Close()
		return nil, err
	}
	var cloud syncUC.Cloud
	if c.CloudSyncURL != "" {
		cloud = syncClient.New(c.CloudSyncURL, c.CloudToken)
	}
	var commands syncUC.CommandCloud
	if c.CloudCommandsURL != "" {
		commands = syncClient.New(c.CloudCommandsURL, c.CloudToken)
	}
	return &Container{ArenaStore: store, Arena: arena, Background: arenaWorker.New(), Synchronization: syncUC.New(store, cloud), Commands: syncUC.NewCommands(store, commands, operationUC.New(streaming, saving), c.ArenaID, c.EngineID), ChannelCatalog: sources, MediaServer: mediaServer, Video: video, Audio: audio, Streaming: streaming, Saving: saving, Handlers: delivery.Handlers{Arena: arenaDelivery.New(arena, strings.HasPrefix(c.LocalPublicURL, "https://")), Video: videoDelivery.New(video), Audio: audioDelivery.New(audio, c.Origins), Streaming: streamingDelivery.New(streaming), Saving: savingDelivery.New(saving)}}, nil
}

type unavailableRecordings struct{}

func (unavailableRecordings) Save(context.Context, savingModel.Recording) error {
	return apperror.ErrUnavailable
}
func (unavailableRecordings) List(context.Context) ([]savingModel.Recording, error) {
	return nil, apperror.ErrUnavailable
}

func mediaSources(catalog *channelRepo.Catalog) []mediamtx.Source {
	inputs := catalog.Inputs()
	sources := make([]mediamtx.Source, 0, len(inputs))
	for _, in := range inputs {
		sources = append(sources, mediamtx.Source{ID: in.ID, URL: in.Source})
	}
	return sources
}
