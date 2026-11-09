// Session operations reserve, start, finalize, stop, and roll back recordings.
package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/mediaengine/media"
	"kicau.local/edge/internal/mediaengine/saving/model"
)

func (u *UseCase) StartSession(ctx context.Context, req model.StartSession) ([]model.Recording, error) {
	// Serialize submissions and shutdown, but never hold business state locks
	// while scheduling or waiting for worker callbacks.
	u.ops.Lock()
	defer u.ops.Unlock()
	records, err := u.reserve(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		source, _ := u.sources.HubRTSP(r.ChannelID)
		p, err := u.recorder.Record(u.executor.Context(), source, r.File+".partial")
		if err != nil {
			u.rollbackStart(records)
			return nil, apperror.ErrUnavailable
		}
		a := &active{p: p, round: r.RoundID}
		u.mu.Lock()
		u.running[r.ID] = a
		u.mu.Unlock()
		done, err := u.executor.Watch(p, func(processErr error) { u.finishRecording(r, processErr) })
		if err != nil {
			p.Stop()
			u.finishRecording(r, err)
			u.rollbackStart(records)
			return nil, apperror.ErrUnavailable
		}
		u.mu.Lock()
		a.finished = done
		u.mu.Unlock()
	}
	return records, nil
}

func (u *UseCase) reserve(ctx context.Context, req model.StartSession) ([]model.Recording, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closing || !u.started {
		return nil, apperror.ErrUnavailable
	}
	if !identifier.Valid(req.ID) || len(req.Channels) == 0 || len(req.Channels) > 24 {
		return nil, apperror.ErrInvalid
	}
	seen := map[string]bool{}
	for _, id := range req.Channels {
		if _, err := u.sources.HubRTSP(id); err != nil {
			return nil, err
		}
		if seen[id] {
			return nil, apperror.ErrInvalid
		}
		seen[id] = true
	}
	if len(u.running) > 0 {
		return nil, apperror.ErrConflict
	}
	all, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range all {
		if r.RoundID == req.ID {
			return nil, apperror.ErrConflict
		}
	}
	free, err := u.disk.Free(u.cfg.DataDir)
	if err != nil || free < u.cfg.MinFreeBytes {
		return nil, apperror.ErrUnavailable
	}
	records := []model.Recording{}
	for _, ch := range req.Channels {
		random := make([]byte, 16)
		if _, err = rand.Read(random); err != nil {
			u.failReservations(records)
			return nil, err
		}
		id := hex.EncodeToString(random)
		r := model.Recording{ID: id, RoundID: req.ID, ChannelID: ch, State: "RECORDING", File: filepath.Join(u.cfg.DataDir, "recordings", id+".mp4"), Key: "rounds/" + req.ID + "/" + ch + "/" + id + ".mp4", StartedAt: time.Now()}
		if err = u.repo.Save(ctx, r); err != nil {
			u.failReservations(records)
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

func (u *UseCase) rollbackStart(records []model.Recording) {
	u.mu.Lock()
	pending := []model.Recording{}
	processes := []media.Process{}
	// A previously started process may have finalized while the next one was
	// being created. Do not overwrite that completed manifest during rollback.
	latest, err := u.repo.List(u.executor.Context())
	states := map[string]string{}
	if err == nil {
		for _, r := range latest {
			states[r.ID] = r.State
		}
	}
	for _, r := range records {
		if _, ok := u.running[r.ID]; !ok && (err != nil || states[r.ID] == "RECORDING") {
			pending = append(pending, r)
		}
	}
	u.failReservations(pending)
	for _, a := range u.running {
		processes = append(processes, a.p)
	}
	u.mu.Unlock()
	if err := u.executor.StopProcesses(processes); err != nil {
		logger.Error("cannot stop recording rollback", zap.Error(err))
	}
}

func (u *UseCase) finishRecording(r model.Recording, processErr error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	defer delete(u.running, r.ID)
	r.EndedAt = time.Now()
	r.State = "FAILED"
	partial := r.File + ".partial"
	if size, err := u.disk.Size(partial); err == nil && size > 0 && processErr == nil {
		if err = u.disk.Commit(partial, r.File); err == nil {
			r.State = "READY"
		}
	}
	if err := u.repo.Save(context.Background(), r); err != nil {
		logger.Error("cannot persist finalized recording; restart recovery required", zap.String("recording", r.ID), zap.Error(err))
	}
	logger.Info("recording completed", zap.String("recording", r.ID), zap.String("state", r.State))
}

func (u *UseCase) StopSession(ctx context.Context, id string) error {
	if !identifier.Valid(id) {
		return apperror.ErrInvalid
	}
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	processes := []media.Process{}
	completions := []<-chan struct{}{}
	for _, a := range u.running {
		if a.round == id {
			processes = append(processes, a.p)
			completions = append(completions, a.finished)
		}
	}
	u.mu.Unlock()
	if len(processes) == 0 {
		return apperror.ErrNotFound
	}
	if err := u.executor.StopProcesses(processes); err != nil {
		return err
	}
	return u.executor.WaitFor(ctx, completions)
}

func (u *UseCase) failReservations(records []model.Recording) {
	for _, r := range records {
		r.State = "FAILED"
		r.EndedAt = time.Now()
		if e := u.repo.Save(u.executor.Context(), r); e != nil {
			logger.Error("cannot persist reservation rollback", zap.String("recording", r.ID), zap.Error(e))
		}
	}
}
