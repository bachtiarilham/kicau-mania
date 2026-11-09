// Playback authorization validates the channel and cloud-issued grant.
package usecase

import (
	"context"
	"kicau.local/edge/internal/apperror"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	"time"
)

func (e *Engine) ValidateChannel(id string) error {
	_, err := e.channels.HubRTSP(id)
	return err
}

func (e *Engine) Authorize(ctx context.Context, channelID, ticket string) error {
	if err := e.ValidateChannel(channelID); err != nil {
		return err
	}
	if ticket == "" || len(ticket) > 8192 {
		return apperror.ErrUnauthorized
	}
	if e.playback == nil {
		return apperror.ErrUnavailable
	}
	var grant cloudmodel.PlaybackGrant
	err := e.playback.Post(ctx, cloudmodel.PlaybackRequest{Ticket: ticket, ArenaID: e.opts.ArenaID, EngineID: e.opts.EngineID, ChannelID: channelID}, &grant)
	if err != nil {
		return apperror.ErrUnavailable
	}
	if !grant.Allowed || grant.UserID == "" || grant.ArenaID != e.opts.ArenaID || grant.EngineID != e.opts.EngineID || grant.ChannelID != channelID || !grant.ExpiresAt.After(time.Now()) || grant.ExpiresAt.After(time.Now().Add(65*time.Second)) {
		return apperror.ErrForbidden
	}
	return nil
}
