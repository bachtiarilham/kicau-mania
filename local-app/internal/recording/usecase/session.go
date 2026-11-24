package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/recording/model"
	streaming "kicau.local/edge/internal/mediaengine/streaming/model"
)

type Journal interface {
	Save(context.Context, model.Session) error
	Get(context.Context, string) (model.Session, error)
	List(context.Context) ([]model.Session, error)
}

type Recordings interface {
	Save(context.Context, model.Recording) error
	List(context.Context) ([]model.Recording, error)
}

type Catalog interface {
	HubRTSP(string) (string, error)
}

type Pipeline interface {
	RunMedia(context.Context, string, string, func(), func([]byte)) error
}

type Finalizer interface {
	Finalize(context.Context, []string, string) error
}

type Audio interface {
	Activate([]string) error
	Deactivate()
	Ingest(string, []byte)
}

type Restreams interface {
	StartRestream(string, string) error
	StopRestream(string) error
	Restreams() []streaming.RestreamView
}

type Disk interface {
	Free(string) (uint64, error)
}

type Worker interface {
	Start() error
	Context() context.Context
	Run(func(context.Context)) error
	Group(context.Context, []func(context.Context), func()) error
	Wait(context.Context, time.Duration) bool
	Close()
}

type Connections interface {
	CloseSessions(context.Context) error
}

type MediaReset interface {
	Restart() error
}

type SessionOptions struct {
	DataDir      string
	MaxDuration  time.Duration
	MinFreeBytes uint64
	Targets      map[string]string
}

type run struct {
	session             model.Session
	ctx                 context.Context
	cancel              context.CancelFunc
	pipelinesDone, done chan struct{}
	records             map[string]model.Recording
}

type SessionUseCase struct {
	opts             SessionOptions
	journal          Journal
	recordings       Recordings
	catalog          Catalog
	pipeline         Pipeline
	finalizer        Finalizer
	audio            Audio
	restreams        Restreams
	disk             Disk
	worker           Worker
	media            MediaReset
	ops              sync.Mutex
	mu               sync.Mutex
	admissions       sync.WaitGroup
	active           *run
	started, closing bool
	live, sockets    Connections
}

func NewSessionUseCase(
	opts SessionOptions,
	journal Journal,
	recordings Recordings,
	catalog Catalog,
	pipeline Pipeline,
	finalizer Finalizer,
	audio Audio,
	restreams Restreams,
	disk Disk,
	worker Worker,
	media MediaReset,
) *SessionUseCase {
	targets := map[string]string{}
	for k, v := range opts.Targets {
		targets[k] = v
	}
	opts.Targets = targets
	return &SessionUseCase{
		opts:       opts,
		journal:    journal,
		recordings: recordings,
		catalog:    catalog,
		pipeline:   pipeline,
		finalizer:  finalizer,
		audio:      audio,
		restreams:  restreams,
		disk:       disk,
		worker:     worker,
		media:      media,
	}
}

func (u *SessionUseCase) BindConnections(live, sockets Connections) {
	u.live, u.sockets = live, sockets
}

func (u *SessionUseCase) Start() error {
	u.ops.Lock()
	defer u.ops.Unlock()
	if u.closing || u.started {
		return apperror.ErrConflict
	}
	if err := os.MkdirAll(filepath.Join(u.opts.DataDir, "recordings"), 0700); err != nil {
		return err
	}
	if err := u.recover(u.worker.Context()); err != nil {
		return err
	}
	if err := u.worker.Start(); err != nil {
		return err
	}
	u.mu.Lock()
	u.started = true
	u.mu.Unlock()
	return nil
}

func (u *SessionUseCase) validate(req model.StartRequest) error {
	if !identifier.Valid(req.ID) || len(req.Channels) < 1 || len(req.Channels) > 24 {
		return apperror.ErrInvalid
	}
	channels := map[string]bool{}
	for _, id := range req.Channels {
		if channels[id] {
			return apperror.ErrInvalid
		}
		if _, err := u.catalog.HubRTSP(id); err != nil {
			return err
		}
		channels[id] = true
	}
	targets := map[string]bool{}
	for _, r := range req.Restreams {
		if !channels[r.ChannelID] || targets[r.Target] || u.opts.Targets[r.Target] == "" {
			return apperror.ErrInvalid
		}
		targets[r.Target] = true
	}
	free, err := u.disk.Free(u.opts.DataDir)
	if err != nil {
		return apperror.Diagnostic(apperror.ErrUnavailable, "DISK_UNAVAILABLE", "Kapasitas disk tidak dapat diperiksa")
	}
	if free < u.opts.MinFreeBytes {
		return apperror.Diagnostic(apperror.ErrUnavailable, "DISK_LOW", "Ruang disk di bawah batas minimum sesi")
	}
	return nil
}

func (u *SessionUseCase) StartSession(ctx context.Context, req model.StartRequest) (model.Session, error) {
	u.ops.Lock()
	defer u.ops.Unlock()
	if err := u.validate(req); err != nil {
		return model.Session{}, err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.started || u.closing {
		return model.Session{}, apperror.ErrUnavailable
	}
	old, err := u.journal.Get(ctx, req.ID)
	if err == nil {
		a, _ := json.Marshal(old.StartRequest)
		b, _ := json.Marshal(req)
		if u.active != nil && u.active.session.ID == req.ID && (old.State == "STARTING" || old.State == "RUNNING" || old.State == "DEGRADED") && string(a) == string(b) {
			return clone(u.active.session), nil
		}
		return model.Session{}, apperror.ErrConflict
	}
	if !errors.Is(err, apperror.ErrNotFound) && err != nil {
		return model.Session{}, err
	}
	if u.active != nil {
		return model.Session{}, apperror.ErrConflict
	}
	all, err := u.recordings.List(ctx)
	if err != nil {
		return model.Session{}, err
	}
	for _, r := range all {
		if r.RoundID == req.ID {
			return model.Session{}, apperror.ErrConflict
		}
	}
	req.Channels = append([]string(nil), req.Channels...)
	req.Restreams = append([]model.Restream(nil), req.Restreams...)
	s := model.Session{StartRequest: req, State: "STARTING", StartedAt: time.Now().UTC(), ChannelsState: []model.Channel{}}
	for _, id := range req.Channels {
		hash := sha256.Sum256([]byte(req.ID + "\x00" + id))
		s.ChannelsState = append(s.ChannelsState, model.Channel{ID: id, RecordingID: hex.EncodeToString(hash[:16]), State: "CONNECTING"})
	}
	if err := u.journal.Save(ctx, s); err != nil {
		return model.Session{}, err
	}
	jobCtx, cancel := context.WithCancel(u.worker.Context())
	r := &run{session: s, ctx: jobCtx, cancel: cancel, pipelinesDone: make(chan struct{}), done: make(chan struct{}), records: map[string]model.Recording{}}
	for _, ch := range s.ChannelsState {
		rec := model.Recording{ID: ch.RecordingID, RoundID: req.ID, ChannelID: ch.ID, State: "RECORDING", File: filepath.Join(u.opts.DataDir, "recordings", ch.RecordingID+".mkv"), Key: "rounds/" + req.ID + "/" + ch.ID + "/" + ch.RecordingID + ".mkv", StartedAt: s.StartedAt}
		if err := u.recordings.Save(ctx, rec); err != nil {
			u.abortLocked(r)
			return model.Session{}, err
		}
		r.records[ch.ID] = rec
	}
	u.active = r
	if err := u.audio.Activate(req.Channels); err != nil {
		u.abortLocked(r)
		return model.Session{}, err
	}
	tasks := []func(context.Context){}
	for _, id := range req.Channels {
		id := id
		tasks = append(tasks, func(ctx context.Context) { u.capture(r, ctx, id) })
	}
	if err := u.worker.Group(jobCtx, tasks, func() { close(r.pipelinesDone) }); err != nil {
		u.abortLocked(r)
		return model.Session{}, err
	}
	if err := u.worker.Run(func(context.Context) { u.guard(r) }); err != nil {
		cancel()
		r.session.Error = "session scheduling failed"
		r.session.State = "STOPPING"
		r.session.StoppedAt = time.Now().UTC()
		_ = u.journal.Save(context.Background(), r.session)
		_ = u.worker.Run(func(context.Context) { u.finish(r) })
		return model.Session{}, err
	}
	return clone(r.session), nil
}

func (u *SessionUseCase) abortLocked(r *run) {
	r.cancel()
	u.audio.Deactivate()
	r.session.State = "FAILED"
	r.session.Error = "session startup failed"
	for _, rec := range r.records {
		rec.State = "FAILED"
		rec.Error = r.session.Error
		_ = u.recordings.Save(context.Background(), rec)
	}
	_ = u.journal.Save(context.Background(), r.session)
	u.active = nil
	close(r.done)
}

func clone(s model.Session) model.Session {
	s.Channels = append([]string(nil), s.Channels...)
	s.Restreams = append([]model.Restream(nil), s.Restreams...)
	s.ChannelsState = append([]model.Channel(nil), s.ChannelsState...)
	for i := range s.ChannelsState {
		s.ChannelsState[i].Gaps = append([]model.Gap(nil), s.ChannelsState[i].Gaps...)
	}
	return s
}

func (u *SessionUseCase) Get(ctx context.Context, id string) (model.Session, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.active != nil && u.active.session.ID == id {
		return clone(u.active.session), nil
	}
	return u.journal.Get(ctx, id)
}

func (u *SessionUseCase) Current(ctx context.Context) (*model.Session, error) {
	u.mu.Lock()
	if u.active != nil {
		s := clone(u.active.session)
		u.mu.Unlock()
		return &s, nil
	}
	u.mu.Unlock()
	all, e := u.journal.List(ctx)
	if e != nil {
		return nil, e
	}
	var latest *model.Session
	for _, s := range all {
		if latest == nil || s.StartedAt.After(latest.StartedAt) {
			v := clone(s)
			latest = &v
		}
	}
	return latest, nil
}

func (u *SessionUseCase) Busy() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.active != nil
}

func (u *SessionUseCase) Reset(ctx context.Context) error {
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.active != nil {
		return apperror.ErrConflict
	}
	sessionDir := filepath.Join(u.opts.DataDir, "media-sessions")
	entries, err := os.ReadDir(sessionDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				_ = os.Remove(filepath.Join(sessionDir, e.Name()))
			}
		}
	}
	queueDir := filepath.Join(u.opts.DataDir, "queue")
	qEntries, qErr := os.ReadDir(queueDir)
	if qErr == nil {
		for _, e := range qEntries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				_ = os.Remove(filepath.Join(queueDir, e.Name()))
			}
		}
	}
	return nil
}

func (u *SessionUseCase) AcquirePlayback(channel string) (func(), error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	r := u.active
	if r == nil || r.ctx.Err() != nil {
		return nil, apperror.ErrConflict
	}
	found := false
	for _, id := range r.session.Channels {
		if id == channel {
			found = true
		}
	}
	if !found {
		return nil, apperror.ErrForbidden
	}
	u.admissions.Add(1)
	var once sync.Once
	return func() { once.Do(u.admissions.Done) }, nil
}

func (u *SessionUseCase) StopSession(ctx context.Context, id string) (model.Session, error) {
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	defer u.mu.Unlock()
	r := u.active
	if r == nil || r.session.ID != id {
		s, err := u.journal.Get(ctx, id)
		if err == nil && model.Terminal(s.State) {
			return s, nil
		}
		if err != nil {
			return s, err
		}
		return s, apperror.ErrConflict
	}
	if r.session.State == "STOPPING" || r.session.State == "FINALIZING" {
		select {
		case <-r.done:
			r.done = make(chan struct{})
			if err := u.worker.Run(func(context.Context) { u.finish(r) }); err != nil {
				close(r.done)
				return clone(r.session), err
			}
		default:
		}
		return clone(r.session), nil
	}
	if model.Terminal(r.session.State) {
		return clone(r.session), nil
	}
	r.session.State = "STOPPING"
	if r.session.StopReason == "" {
		r.session.StopReason = "OPERATOR"
	}
	r.session.StoppedAt = time.Now().UTC()
	r.cancel()
	u.audio.Deactivate()
	persistErr := u.journal.Save(context.Background(), r.session)
	if err := u.worker.Run(func(context.Context) { u.finish(r) }); err != nil {
		return clone(r.session), err
	}
	return clone(r.session), persistErr
}

func (u *SessionUseCase) Close() {
	u.mu.Lock()
	u.closing = true
	r := u.active
	u.mu.Unlock()
	if r != nil {
		_, err := u.StopSession(context.Background(), r.session.ID)
		if err != nil {
			logger.Error("session shutdown persistence failed", zap.Error(err))
		}
		u.mu.Lock()
		done := r.done
		u.mu.Unlock()
		<-done
	}
	u.worker.Close()
}

func (u *SessionUseCase) guard(r *run) {
	for r.ctx.Err() == nil {
		free, err := u.disk.Free(u.opts.DataDir)
		if err != nil || free < u.opts.MinFreeBytes || time.Since(r.session.StartedAt) >= u.opts.MaxDuration {
			u.mu.Lock()
			if r.ctx.Err() == nil {
				reason := "MAX_DURATION"
				if err != nil {
					reason = "DISK_UNAVAILABLE"
				} else if free < u.opts.MinFreeBytes {
					reason = "DISK_LOW"
				}
				r.session.StopReason = reason
			}
			u.mu.Unlock()
			_, _ = u.StopSession(context.Background(), r.session.ID)
			return
		}
		u.ops.Lock()
		views := u.restreams.Restreams()
		for _, target := range r.session.Restreams {
			if r.ctx.Err() != nil {
				break
			}
			found := false
			for _, v := range views {
				if v.Target == target.Target {
					found = true
				}
			}
			if !found {
				_ = u.restreams.StartRestream(target.ChannelID, target.Target)
			}
		}
		u.ops.Unlock()
		u.mu.Lock()
		if r.ctx.Err() == nil {
			u.updateLocked(r)
			for _, target := range r.session.Restreams {
				running := false
				for _, v := range u.restreams.Restreams() {
					if v.Target == target.Target && v.State == "RUNNING" {
						running = true
					}
				}
				if !running {
					r.session.State = "DEGRADED"
				}
			}
			_ = u.journal.Save(context.Background(), r.session)
		}
		u.mu.Unlock()
		if !u.worker.Wait(r.ctx, 5*time.Second) {
			return
		}
	}
}
