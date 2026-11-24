// Recording metadata is journaled and sent until cloud acknowledges each update.
package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	saving "kicau.local/edge/internal/recording/model"
)

func (e *Engine) allRecordings(ctx context.Context) ([]saving.Recording, error) {
	all := []saving.Recording{}
	cursor := ""
	for {
		page, next, err := e.recordings.Recordings(ctx, cursor, 100)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if next == "" {
			return all, nil
		}
		cursor = next
	}
}

func (e *Engine) PublishMetadata(ctx context.Context) error {
	items, err := e.allRecordings(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		update := cloudmodel.MediaUpdate{ArenaID: e.opts.ArenaID, EngineID: e.opts.EngineID, RecordingID: item.ID, SessionID: item.RoundID, ChannelID: item.ChannelID, Version: item.Version, State: item.State, ObjectKey: item.Key, StartedAt: item.StartedAt, EndedAt: item.EndedAt}
		body, err := json.Marshal(update)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(body)
		update.ID = hex.EncodeToString(hash[:])
		if err := e.updates.Enqueue(ctx, update); err != nil {
			return err
		}
	}
	batch, err := e.updates.Pending(ctx)
	if err != nil || len(batch) == 0 {
		return err
	}
	if e.metadata == nil {
		return fmt.Errorf("media update endpoint not configured")
	}
	var response struct {
		Acknowledged []string `json:"acknowledged"`
	}
	if err := e.metadata.Post(ctx, struct {
		Updates []cloudmodel.MediaUpdate `json:"updates"`
	}{batch}, &response); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, item := range batch {
		allowed[item.ID] = true
	}
	for _, id := range response.Acknowledged {
		if !allowed[id] {
			return fmt.Errorf("media update acknowledgment mismatch")
		}
		delete(allowed, id)
	}
	return e.updates.Acknowledge(ctx, response.Acknowledged)
}
