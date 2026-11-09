package usecase

import (
	"context"
	"errors"
	"fmt"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/mediaengine/media"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"kicau.local/edge/internal/infra/logger"
	recording "kicau.local/edge/internal/mediaengine/saving/model"
	"kicau.local/edge/internal/mediaengine/session/model"
)

func (u *UseCase) capture(r *run, ctx context.Context, id string) {
	var gap time.Time
	defer func() {
		if !gap.IsZero() {
			u.mu.Lock()
			defer u.mu.Unlock()
			rec := r.records[id]
			end := r.session.StoppedAt
			if end.IsZero() {
				end = time.Now().UTC()
			}
			rec.Gaps = append(rec.Gaps, recording.Gap{Reason: "source_interruption", StartedAt: gap, EndedAt: end})
			r.records[id] = rec
			_ = u.recordings.Save(context.Background(), rec)
		}
	}()
	for ctx.Err() == nil {
		u.mu.Lock()
		rec := r.records[id]
		attempt := len(rec.Segments) + 1
		part := fmt.Sprintf("%s.segment-%06d.mp4.partial", rec.File, attempt)
		rec.Segments = append(rec.Segments, recording.Segment{File: part, StartedAt: time.Now().UTC()})
		r.records[id] = rec
		for i := range r.session.ChannelsState {
			if r.session.ChannelsState[i].ID == id {
				r.session.ChannelsState[i].State = "CONNECTING"
				r.session.ChannelsState[i].Attempts = attempt
			}
		}
		err := u.recordings.Save(context.Background(), rec)
		u.updateLocked(r)
		u.mu.Unlock()
		if err != nil {
			_, _ = u.StopSession(context.Background(), r.session.ID)
			return
		}
		source, _ := u.catalog.HubRTSP(id)
		var first sync.Once
		err = u.pipeline.Run(ctx, source, part, func(pcm []byte) {
			first.Do(func() {
				u.mu.Lock()
				defer u.mu.Unlock()
				v := r.records[id]
				v.Segments[len(v.Segments)-1].HasMedia = true
				if !gap.IsZero() {
					v.Gaps = append(v.Gaps, recording.Gap{Reason: "source_interruption", StartedAt: gap, EndedAt: time.Now().UTC()})
					gap = time.Time{}
				}
				r.records[id] = v
				for i := range r.session.ChannelsState {
					if r.session.ChannelsState[i].ID == id {
						r.session.ChannelsState[i].State = "RUNNING"
						r.session.ChannelsState[i].Error = ""
					}
				}
				u.updateLocked(r)
			})
			u.audio.Ingest(id, pcm)
		})
		u.mu.Lock()
		rec = r.records[id]
		last := &rec.Segments[len(rec.Segments)-1]
		last.EndedAt = time.Now().UTC()
		if ctx.Err() == nil {
			if gap.IsZero() {
				gap = last.EndedAt
				if !last.HasMedia {
					gap = last.StartedAt
				}
			}
			for i := range r.session.ChannelsState {
				if r.session.ChannelsState[i].ID == id {
					r.session.ChannelsState[i].State = "RETRYING"
					r.session.ChannelsState[i].Error = "media pipeline interrupted"
				}
			}
		}
		if ctx.Err() != nil && !gap.IsZero() {
			rec.Gaps = append(rec.Gaps, recording.Gap{Reason: "source_interruption", StartedAt: gap, EndedAt: last.EndedAt})
			gap = time.Time{}
		}
		r.records[id] = rec
		saveErr := u.recordings.Save(context.Background(), rec)
		u.updateLocked(r)
		u.mu.Unlock()
		if saveErr != nil {
			_, _ = u.StopSession(context.Background(), r.session.ID)
			return
		}
		if err != nil && ctx.Err() == nil {
			logger.Warn("session channel interrupted; retrying", zap.String("channel", id))
		}
		if !u.worker.Wait(ctx, 5*time.Second) {
			return
		}
	}
}
func (u *UseCase) safePath(path string) bool {
	root, err := filepath.Abs(filepath.Join(u.opts.DataDir, "recordings"))
	if err != nil {
		return false
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, absolute)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func (u *UseCase) finalize(ctx context.Context, rec recording.Recording, at time.Time) error {
	if rec.State == "READY" || rec.State == "UPLOADED" {
		return nil
	}
	rec.EndedAt = at
	if !u.safePath(rec.File) {
		rec.State = "FAILED"
		rec.Error = "recording path outside data directory"
		return u.recordings.Save(ctx, rec)
	}
	parts := []string{}
	for _, segment := range rec.Segments {
		if !u.safePath(segment.File) {
			rec.State = "FAILED"
			rec.Error = "recording segment path outside data directory"
			return u.recordings.Save(ctx, rec)
		}
		// PCM delivery is best effort; it is not proof that a segment is empty.
		// Include any bytes written and let the finalizer validate them. Only
		// attempts that never wrote media bytes can be skipped as recorded gaps.
		st, statErr := os.Stat(segment.File)
		if segment.HasMedia || statErr == nil && st.Size() > 0 {
			parts = append(parts, segment.File)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			rec.State = "FAILED"
			rec.Error = "recording segment unavailable"
			return u.recordings.Save(ctx, rec)
		}
	}
	if len(rec.Segments) == 0 {
		if _, err := os.Stat(rec.File); err == nil {
			parts = []string{rec.File}
		} else if _, err := os.Stat(rec.File + ".partial"); err == nil {
			parts = []string{rec.File + ".partial"}
		}
	}
	finalCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	err := u.finalizer.Finalize(finalCtx, parts, rec.File)
	rec.EndedAt = at
	rec.State = "READY"
	rec.Error = ""
	if err != nil {
		rec.State = "FAILED"
		rec.Error = "recording validation or finalization failed"
		var reason media.FinalizationError
		if errors.As(err, &reason) {
			rec.Error = string(reason)
		}
	}
	return u.recordings.Save(ctx, rec)
}
func (u *UseCase) finish(r *run) {
	defer close(r.done)
	// Stop admission was committed before scheduling this function. All POSTs
	// already admitted must complete registration before connection teardown.
	u.admissions.Wait()
	u.audio.Deactivate()
	u.mu.Lock()
	retryPlaybackReset := r.session.Error == "playback teardown failed"
	u.mu.Unlock()
	playbackFailed := false
	if retryPlaybackReset {
		playbackFailed = u.media.Restart() != nil
	}
	for _, connections := range []Connections{u.live, u.sockets} {
		if connections == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := connections.CloseSessions(ctx)
		cancel()
		if err != nil {
			if resetErr := u.media.Restart(); resetErr != nil {
				playbackFailed = true
			}
		}
	}
	restreamFailed := false
	for _, target := range r.session.Restreams {
		if err := u.restreams.StopRestream(target.Target); err != nil && !errors.Is(err, apperror.ErrNotFound) {
			restreamFailed = true
		}
	}
	<-r.pipelinesDone
	if playbackFailed || restreamFailed {
		u.mu.Lock()
		r.session.Error = "restream teardown failed"
		if playbackFailed {
			r.session.Error = "playback teardown failed"
		}
		_ = u.journal.Save(context.Background(), r.session)
		u.mu.Unlock()
		logger.Error("media teardown incomplete; retry Stop required")
		return
	}
	u.mu.Lock()
	r.session.Error = ""
	r.session.State = "FINALIZING"
	for i := range r.session.ChannelsState {
		r.session.ChannelsState[i].State = "STOPPED"
		r.session.ChannelsState[i].RecordingState = "FINALIZING"
	}
	records := []recording.Recording{}
	for _, rec := range r.records {
		rec.State = "FINALIZING"
		records = append(records, rec)
	}
	_ = u.journal.Save(context.Background(), r.session)
	u.mu.Unlock()
	failed := false
	for _, rec := range records {
		// A previous completion pass may already have committed/uploaded a result.
		// Never overwrite its upload attempts/state when retrying journal completion.
		current, readErr := u.recordings.List(context.Background())
		if readErr != nil {
			failed = true
			continue
		}
		for _, saved := range current {
			if saved.ID == rec.ID && (saved.State == "READY" || saved.State == "UPLOADED") {
				rec = saved
			}
		}
		if err := u.finalize(context.Background(), rec, r.session.StoppedAt); err != nil {
			failed = true
			logger.Error("recording final state persistence failed", zap.Error(err))
			continue
		}
		all, err := u.recordings.List(context.Background())
		if err != nil {
			failed = true
			continue
		}
		u.mu.Lock()
		for i := range r.session.ChannelsState {
			if r.session.ChannelsState[i].ID == rec.ChannelID {
				r.session.ChannelsState[i].State = "STOPPED"
				for _, current := range all {
					if current.ID == rec.ID {
						r.session.ChannelsState[i].RecordingState = current.State
						r.session.ChannelsState[i].SegmentCount = len(current.Segments)
						r.session.ChannelsState[i].Gaps = nil
						for _, gap := range current.Gaps {
							r.session.ChannelsState[i].Gaps = append(r.session.ChannelsState[i].Gaps, model.Gap{StartedAt: gap.StartedAt, EndedAt: gap.EndedAt, Reason: gap.Reason})
						}
					}
					if current.ID == rec.ID && current.State == "FAILED" {
						r.session.ChannelsState[i].State = "FAILED"
						r.session.ChannelsState[i].Error = current.Error
					}
				}
			}
		}
		u.mu.Unlock()
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if failed {
		r.session.Error = "recording state persistence failed"
		r.session.State = "FINALIZING"
	} else {
		r.session.State = "STOPPED"
	}
	if err := u.journal.Save(context.Background(), r.session); err != nil {
		r.session.Error = "session completion persistence failed"
		r.session.State = "FINALIZING"
		logger.Error("session completion persistence failed", zap.Error(err))
		return
	}
	if !failed && u.active == r {
		u.active = nil
	}
}
func (u *UseCase) recover(ctx context.Context) error {
	sessions, err := u.journal.List(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, s := range sessions {
		if model.Terminal(s.State) {
			continue
		}
		s.State = "INTERRUPTED"
		s.StoppedAt = now
		s.Error = "engine restarted; media not resumed"
		for i := range s.ChannelsState {
			s.ChannelsState[i].State = "STOPPED"
		}
		if err := u.journal.Save(ctx, s); err != nil {
			return err
		}
	}
	all, err := u.recordings.List(ctx)
	if err != nil {
		return err
	}
	for _, rec := range all {
		if rec.State != "RECORDING" && rec.State != "FINALIZING" {
			continue
		}
		for i := range rec.Segments {
			segment := &rec.Segments[i]
			if segment.EndedAt.IsZero() {
				segment.EndedAt = now
				segment.HasMedia = true
				rec.Gaps = append(rec.Gaps, recording.Gap{Reason: "restart_uncertain", StartedAt: segment.StartedAt, EndedAt: now})
			}
		}
		if err := u.finalize(ctx, rec, now); err != nil {
			return err
		}
	}
	// Publish the finalization outcome in interrupted session status as well as
	// in the upload manifest. Recovery never reopens media admission.
	all, err = u.recordings.List(ctx)
	if err != nil {
		return err
	}
	for _, previous := range sessions {
		if model.Terminal(previous.State) {
			continue
		}
		current, err := u.journal.Get(ctx, previous.ID)
		if err != nil {
			return err
		}
		for i := range current.ChannelsState {
			ch := &current.ChannelsState[i]
			for _, rec := range all {
				if rec.RoundID != current.ID || rec.ChannelID != ch.ID {
					continue
				}
				ch.RecordingState = rec.State
				ch.SegmentCount = len(rec.Segments)
				ch.Gaps = nil
				for _, gap := range rec.Gaps {
					ch.Gaps = append(ch.Gaps, model.Gap{StartedAt: gap.StartedAt, EndedAt: gap.EndedAt, Reason: gap.Reason})
				}
				if rec.State == "FAILED" {
					ch.State = "FAILED"
					ch.Error = rec.Error
				}
			}
		}
		if err = u.journal.Save(ctx, current); err != nil {
			return err
		}
	}
	return nil
}
