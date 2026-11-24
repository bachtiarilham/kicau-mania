// Package usecase coordinates cloud communication and media operations through interfaces.
package usecase

import (
	"context"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	channel "kicau.local/edge/internal/camera/model"
	"kicau.local/edge/internal/mediaengine/media"
	saving "kicau.local/edge/internal/recording/model"
	streaming "kicau.local/edge/internal/mediaengine/streaming/model"
	"sync"
	"time"
)

type Poster interface {
	Post(context.Context, any, any) error
}
type MediaUpdates interface {
	Enqueue(context.Context, cloudmodel.MediaUpdate) error
	Pending(context.Context) ([]cloudmodel.MediaUpdate, error)
	Acknowledge(context.Context, []string) error
}
type Recordings interface {
	Recordings(context.Context, string, int) ([]saving.Recording, string, error)
}
type Restreams interface {
	Restreams() []streaming.RestreamView
}
type MediaStatus interface{ MediaStatus() media.Status }
type Channels interface {
	Channels() []channel.Channel
	HubRTSP(string) (string, error)
}
type EngineOptions struct {
	LAN                bool
	ArenaID, EngineID  string
	HeartbeatTimeout   time.Duration
	CommandsConfigured bool
}
type Engine struct {
	opts                EngineOptions
	channels            Channels
	media               MediaStatus
	recordings          Recordings
	restreams           Restreams
	updates             MediaUpdates
	heartbeat, metadata Poster
	mu                  sync.Mutex
	lastHeartbeat       time.Time
	heartbeatFailed     bool
}

func NewEngine(opts EngineOptions, channels Channels, server MediaStatus, recordings Recordings, restreams Restreams, updates MediaUpdates, heartbeat, metadata Poster) *Engine {
	return &Engine{opts: opts, channels: channels, media: server, recordings: recordings, restreams: restreams, updates: updates, heartbeat: heartbeat, metadata: metadata}
}
