package app

import (
	"context"
	"path/filepath"

	"kicau.local/edge/config"
	audioDelivery "kicau.local/edge/internal/audio/delivery"
	"kicau.local/edge/internal/audio/metricsclient"
	"kicau.local/edge/internal/audio/pubsub"
	audioUC "kicau.local/edge/internal/audio/usecase"
	audioWorker "kicau.local/edge/internal/audio/worker"
	channelRepo "kicau.local/edge/internal/channel/repository"
	delivery "kicau.local/edge/internal/delivery/http"
	"kicau.local/edge/internal/media/ffmpeg"
	"kicau.local/edge/internal/media/mediamtx"
	savingDelivery "kicau.local/edge/internal/saving/delivery"
	savingRepo "kicau.local/edge/internal/saving/repository"
	savingUC "kicau.local/edge/internal/saving/usecase"
	savingWorker "kicau.local/edge/internal/saving/worker"
	"kicau.local/edge/internal/storage/filesystem"
	"kicau.local/edge/internal/storage/s3"
	streamingDelivery "kicau.local/edge/internal/streaming/delivery"
	streamingUC "kicau.local/edge/internal/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/streaming/worker"
	videoDelivery "kicau.local/edge/internal/video/delivery"
	videoUC "kicau.local/edge/internal/video/usecase"
)

type Container struct {
	ChannelCatalog *channelRepo.Catalog
	MediaServer    *mediamtx.Server
	Video          *videoUC.UseCase
	Audio          *audioUC.UseCase
	Streaming      *streamingUC.UseCase
	Saving         *savingUC.UseCase
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
	repo, e := savingRepo.NewFiles(filepath.Join(c.DataDir, "queue"))
	if e != nil {
		return nil, e
	}
	var objectStorage savingUC.Storage
	if c.S3Bucket != "" {
		objectStorage, e = s3.New(ctx, s3.Options{Endpoint: c.S3Endpoint, Region: c.S3Region, Bucket: c.S3Bucket, AccessKey: c.S3AccessKey, SecretKey: c.S3SecretKey})
		if e != nil {
			return nil, e
		}
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
	return &Container{ChannelCatalog: sources, MediaServer: mediaServer, Video: video, Audio: audio, Streaming: streaming, Saving: saving, Handlers: delivery.Handlers{Video: videoDelivery.New(video), Audio: audioDelivery.New(audio, c.Origins), Streaming: streamingDelivery.New(streaming), Saving: savingDelivery.New(saving)}}, nil
}

func mediaSources(catalog *channelRepo.Catalog) []mediamtx.Source {
	inputs := catalog.Inputs()
	sources := make([]mediamtx.Source, 0, len(inputs))
	for _, in := range inputs {
		sources = append(sources, mediamtx.Source{ID: in.ID, URL: in.Source})
	}
	return sources
}
