package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/mediaengine/media"
	"kicau.local/edge/internal/mediaengine/saving/model"
)

type Options struct {
	DataDir      string
	MaxRound     time.Duration
	MinFreeBytes uint64
}
type ChannelRepository interface{ HubRTSP(string) (string, error) }
type RecordingRepository interface {
	Save(context.Context, model.Recording) error
	List(context.Context) ([]model.Recording, error)
}
type Recorder interface {
	Record(context.Context, string, string) (media.Process, error)
}
type active struct {
	p        media.Process
	finished <-chan struct{}
	round    string
}
type Storage interface {
	Put(context.Context, string, string) error
}
type Disk interface {
	Prepare(string) error
	Size(string) (int64, error)
	Commit(string, string) error
	Free(string) (uint64, error)
}
type Executor interface {
	Start() error
	Context() context.Context
	Watch(media.Process, func(error)) (<-chan struct{}, error)
	StartUploadLoop(func(context.Context)) error
	RunGuard(func(context.Context)) error
	UploadWithTimeout(context.Context, func(context.Context) error) error
	StopProcesses([]media.Process) error
	WaitFor(context.Context, []<-chan struct{}) error
	Close()
}
type UseCase struct {
	cfg              Options
	sources          ChannelRepository
	repo             RecordingRepository
	recorder         Recorder
	storage          Storage
	disk             Disk
	executor         Executor
	ops              sync.Mutex
	mu               sync.Mutex
	running          map[string]*active
	closing, started bool
	closeOnce        sync.Once
}

func New(opts Options, sources ChannelRepository, repo RecordingRepository, recorder Recorder, storage Storage, disk Disk, executor Executor) *UseCase {
	return &UseCase{cfg: opts, sources: sources, repo: repo, recorder: recorder, storage: storage, disk: disk, executor: executor, running: map[string]*active{}}
}
func (u *UseCase) Start() error {
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	closing, started := u.closing, u.started
	u.mu.Unlock()
	if closing {
		return apperror.ErrUnavailable
	}
	if started {
		return apperror.ErrConflict
	}
	if err := u.recover(u.executor.Context()); err != nil {
		return err
	}
	if err := u.executor.Start(); err != nil {
		return err
	}
	if u.storage != nil {
		if err := u.executor.StartUploadLoop(u.uploadPending); err != nil {
			u.abortStartup()
			return err
		}
	}
	if err := u.executor.RunGuard(u.checkLimits); err != nil {
		u.abortStartup()
		return err
	}
	u.mu.Lock()
	u.started = true
	u.mu.Unlock()
	return nil
}
func (u *UseCase) abortStartup() {
	u.mu.Lock()
	u.closing = true
	u.mu.Unlock()
	u.executor.Close()
}

func (u *UseCase) recover(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if err := u.disk.Prepare(filepath.Join(u.cfg.DataDir, "recordings")); err != nil {
		return err
	}
	records, err := u.repo.List(ctx)
	if err != nil {
		return err
	}
	// The final filename is the commit marker for a cleanly finalized MP4.
	for _, r := range records {
		if r.State == "RECORDING" {
			r.State = "INTERRUPTED"
			r.EndedAt = time.Now()
			if size, err := u.disk.Size(r.File); err == nil && size > 0 {
				r.State = "READY"
			}
			if err := u.repo.Save(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}
func (u *UseCase) StartRound(ctx context.Context, req model.StartRound) ([]model.Recording, error) {
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
func (u *UseCase) reserve(ctx context.Context, req model.StartRound) ([]model.Recording, error) {
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
		slog.Error("cannot stop recording rollback")
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
		slog.Error("cannot persist finalized recording; restart recovery required", "recording", r.ID)
	}
	slog.Info("recording completed", "recording", r.ID, "state", r.State)
}
func (u *UseCase) StopRound(ctx context.Context, id string) error {
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

func (u *UseCase) Recordings(ctx context.Context, cursor string, limit int) ([]model.Recording, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", apperror.ErrInvalid
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	all, e := u.repo.List(ctx)
	if e != nil {
		return nil, "", e
	}
	out := []model.Recording{}
	next := ""
	for _, r := range all {
		if r.ID <= cursor {
			continue
		}
		if len(out) == limit {
			next = out[len(out)-1].ID
			break
		}
		out = append(out, r)
	}
	return out, next, nil
}

// Session resolves media recording metadata, independently of judging rounds.
func (u *UseCase) Session(ctx context.Context, id string) ([]model.Recording, error) {
	if !identifier.Valid(id) {
		return nil, apperror.ErrInvalid
	}
	all, err := u.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []model.Recording{}
	for _, r := range all {
		if r.RoundID == id {
			out = append(out, r)
		}
	}
	return out, nil
}

func (u *UseCase) uploadPending(ctx context.Context) {
	u.mu.Lock()
	records, err := u.repo.List(ctx)
	u.mu.Unlock()
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("upload queue read failed")
		}
		return
	}
	activeRounds := map[string]bool{}
	for _, r := range records {
		if r.State == "RECORDING" {
			activeRounds[r.RoundID] = true
		}
	}
	for _, r := range records {
		if ctx.Err() != nil {
			return
		}
		if r.State != "READY" || activeRounds[r.RoundID] || time.Now().Before(r.NextAttempt) {
			continue
		}
		err = u.executor.UploadWithTimeout(ctx, func(uploadCtx context.Context) error { return u.storage.Put(uploadCtx, r.Key, r.File) })
		if ctx.Err() != nil {
			return
		}
		r.Attempts++
		if err == nil {
			r.State = "UPLOADED"
		} else {
			shift := min(r.Attempts, 8)
			r.NextAttempt = time.Now().Add(time.Duration(1<<shift) * time.Second)
			slog.Warn("recording upload failed; queued for retry", "recording", r.ID)
		}
		u.mu.Lock()
		err = u.repo.Save(ctx, r)
		u.mu.Unlock()
		if err != nil {
			slog.Error("upload state persistence failed", "recording", r.ID)
		}
	}
}
func (u *UseCase) checkLimits(ctx context.Context) {
	free, err := u.disk.Free(u.cfg.DataDir)
	u.mu.Lock()
	records, readErr := u.repo.List(ctx)
	expired := map[string]bool{}
	if readErr == nil {
		for _, r := range records {
			if r.State == "RECORDING" && time.Since(r.StartedAt) >= u.cfg.MaxRound {
				expired[r.RoundID] = true
			}
		}
	}
	processes := []media.Process{}
	for _, a := range u.running {
		if err != nil || free < u.cfg.MinFreeBytes || expired[a.round] {
			slog.Warn("stopping recording: capacity or time limit", "round", a.round)
			processes = append(processes, a.p)
		}
	}
	u.mu.Unlock()
	if err := u.executor.StopProcesses(processes); err != nil && ctx.Err() == nil {
		slog.Error("cannot schedule recording stop")
	}
}

func (u *UseCase) failReservations(records []model.Recording) {
	for _, r := range records {
		r.State = "FAILED"
		r.EndedAt = time.Now()
		if e := u.repo.Save(u.executor.Context(), r); e != nil {
			slog.Error("cannot persist reservation rollback", "recording", r.ID)
		}
	}
}

func (u *UseCase) Close() {
	u.closeOnce.Do(func() {
		u.ops.Lock()
		defer u.ops.Unlock()
		u.mu.Lock()
		u.closing = true
		processes := []media.Process{}
		completions := []<-chan struct{}{}
		for _, a := range u.running {
			processes = append(processes, a.p)
			completions = append(completions, a.finished)
		}
		u.mu.Unlock()
		if err := u.executor.StopProcesses(processes); err != nil {
			slog.Error("cannot schedule recording shutdown")
		}
		// Do not cancel the worker context until all finalization callbacks have returned.
		_ = u.executor.WaitFor(context.Background(), completions)
		u.executor.Close()
	})
}
