// Engine status reports media readiness and cloud connection freshness.
package usecase

import (
	"kicau.local/edge/internal/mediaengine/media"
	"time"
)

type EngineStatus struct {
	ArenaID        string       `json:"arena_id"`
	EngineID       string       `json:"engine_id"`
	Ready          bool         `json:"ready"`
	CloudConnected bool         `json:"cloud_connected"`
	LastHeartbeat  time.Time    `json:"last_heartbeat,omitempty"`
	Media          media.Status `json:"media"`
}

func (e *Engine) Status() EngineStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	connected := !e.heartbeatFailed && !e.lastHeartbeat.IsZero() && time.Since(e.lastHeartbeat) < e.opts.HeartbeatTimeout
	status := e.media.MediaStatus()
	ready := status.State == "RUNNING" && (e.opts.LAN || connected && e.opts.CommandsConfigured && e.metadata != nil && e.playback != nil)
	return EngineStatus{ArenaID: e.opts.ArenaID, EngineID: e.opts.EngineID, Ready: ready, CloudConnected: connected, LastHeartbeat: e.lastHeartbeat, Media: status}
}
