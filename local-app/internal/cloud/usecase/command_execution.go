// Command execution coordinates recording and restreaming without changing judging state.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"kicau.local/edge/internal/apperror"
	cloudmodel "kicau.local/edge/internal/cloud/model"
	"kicau.local/edge/internal/core/identifier"
	saving "kicau.local/edge/internal/mediaengine/saving/model"
	streaming "kicau.local/edge/internal/mediaengine/streaming/model"
)

type Streaming interface {
	StartRestream(string, string) error
	StopRestream(string) error
	Restreams() []streaming.RestreamView
}
type Saving interface {
	StartSession(context.Context, saving.StartSession) ([]saving.Recording, error)
	StopSession(context.Context, string) error
	Session(context.Context, string) ([]saving.Recording, error)
}
type CommandExecutor struct {
	streaming Streaming
	saving    Saving
}

func NewCommandExecutor(streaming Streaming, saving Saving) *CommandExecutor {
	return &CommandExecutor{streaming: streaming, saving: saving}
}

// Reconcile checks uncertain commands after a restart without starting a new job.
func (u *CommandExecutor) Reconcile(ctx context.Context, command cloudmodel.Command) error {
	var req struct {
		Channel  string   `json:"channel_id"`
		Target   string   `json:"target"`
		Session  string   `json:"session_id"`
		Channels []string `json:"channels"`
	}
	if err := json.Unmarshal(command.Payload, &req); err != nil {
		return apperror.ErrInvalid
	}
	switch command.Kind {
	case "restream.stop", "recording.stop":
		return u.Execute(ctx, command)
	case "restream.start":
		for _, stream := range u.streaming.Restreams() {
			if stream.Target == req.Target && stream.Channel == req.Channel {
				return nil
			}
		}
	case "recording.start":
		recordings, err := u.saving.Session(ctx, req.Session)
		if err != nil {
			return err
		}
		if len(recordings) != len(req.Channels) || len(recordings) == 0 {
			return apperror.ErrConflict
		}
		wanted := map[string]bool{}
		for _, channel := range req.Channels {
			wanted[channel] = true
		}
		for _, recording := range recordings {
			if !wanted[recording.ChannelID] || recording.State != "RECORDING" {
				return apperror.ErrConflict
			}
			delete(wanted, recording.ChannelID)
		}
		return nil
	}
	return apperror.Public(apperror.ErrConflict, "Interrupted command outcome requires operator review.")
}
func (u *CommandExecutor) Execute(ctx context.Context, command cloudmodel.Command) error {
	var req struct {
		Channel  string   `json:"channel_id"`
		Target   string   `json:"target"`
		Session  string   `json:"session_id"`
		Channels []string `json:"channels"`
	}
	if err := json.Unmarshal(command.Payload, &req); err != nil {
		return apperror.ErrInvalid
	}
	if command.Kind == "restream.start" || command.Kind == "restream.stop" {
		if !identifier.Valid(req.Target) {
			return apperror.ErrInvalid
		}
	}
	if command.Kind == "restream.start" && !identifier.Valid(req.Channel) {
		return apperror.ErrInvalid
	}
	if command.Kind == "recording.start" || command.Kind == "recording.stop" {
		if !identifier.Valid(req.Session) {
			return apperror.ErrInvalid
		}
	}
	switch command.Kind {
	case "restream.start":
		for _, stream := range u.streaming.Restreams() {
			if stream.Target == req.Target {
				if stream.Channel == req.Channel {
					return nil
				}
				return apperror.ErrConflict
			}
		}
		return u.streaming.StartRestream(req.Channel, req.Target)
	case "restream.stop":
		err := u.streaming.StopRestream(req.Target)
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	case "recording.start":
		recordings, err := u.saving.Session(ctx, req.Session)
		if err != nil {
			return err
		}
		if len(recordings) > 0 {
			channels := []string{}
			for _, r := range recordings {
				if r.State != "RECORDING" {
					return apperror.ErrConflict
				}
				channels = append(channels, r.ChannelID)
			}
			wanted := append([]string(nil), req.Channels...)
			sort.Strings(channels)
			sort.Strings(wanted)
			if len(channels) != len(wanted) {
				return apperror.ErrConflict
			}
			for i := range channels {
				if channels[i] != wanted[i] {
					return apperror.ErrConflict
				}
			}
			return nil
		}
		_, err = u.saving.StartSession(ctx, saving.StartSession{ID: req.Session, Channels: req.Channels})
		return err
	case "recording.stop":
		err := u.saving.StopSession(ctx, req.Session)
		if errors.Is(err, apperror.ErrNotFound) {
			return nil
		}
		return err
	default:
		return apperror.ErrForbidden
	}
}
