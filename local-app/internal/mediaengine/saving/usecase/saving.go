// Package usecase coordinates recording sessions, recovery, upload, and lifecycle through interfaces.
package usecase

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/logger"
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
			logger.Error("cannot schedule recording shutdown", zap.Error(err))
		}
		// Do not cancel the worker context until all finalization callbacks have returned.
		_ = u.executor.WaitFor(context.Background(), completions)
		u.executor.Close()
	})
}
