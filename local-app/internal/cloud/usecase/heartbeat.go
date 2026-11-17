// Heartbeat sends operational status and records successful cloud acknowledgments.
package usecase

import (
	"context"
	"fmt"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	"time"
)

func (e *Engine) Heartbeat(ctx context.Context) error {
	if e.heartbeat == nil {
		return fmt.Errorf("heartbeat endpoint not configured")
	}
	items, err := e.allRecordings(ctx)
	if err != nil {
		return err
	}
	heartbeat := cloudmodel.Heartbeat{ArenaID: e.opts.ArenaID, EngineID: e.opts.EngineID, At: time.Now().UTC(), MediaState: e.media.MediaStatus().State, Channels: []string{}, RestreamCount: len(e.restreams.Restreams())}
	for _, channel := range e.channels.Channels() {
		heartbeat.Channels = append(heartbeat.Channels, channel.ID)
	}
	for _, item := range items {
		if item.State == "RECORDING" {
			heartbeat.RecordingCount++
		}
		if item.State == "READY" {
			heartbeat.PendingUploads++
		}
	}
	var response struct {
		Acknowledged bool   `json:"acknowledged"`
		ArenaID      string `json:"arena_id"`
		EngineID     string `json:"engine_id"`
	}
	err = e.heartbeat.Post(ctx, heartbeat, &response)
	if err == nil && (!response.Acknowledged || response.ArenaID != e.opts.ArenaID || response.EngineID != e.opts.EngineID) {
		err = fmt.Errorf("heartbeat acknowledgment mismatch")
	}
	e.mu.Lock()
	e.heartbeatFailed = err != nil
	if err == nil {
		e.lastHeartbeat = time.Now().UTC()
	}
	e.mu.Unlock()
	return err
}
