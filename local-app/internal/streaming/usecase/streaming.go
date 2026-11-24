package usecase

import (
	"context"
	"sync"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/media"
	"kicau.local/edge/internal/streaming/model"
)

type ChannelRepository interface {
	HubRTSP(string) (string, error)
}

type Restreamer interface {
	Restream(context.Context, string, string) (media.Process, error)
}

type relay struct {
	view model.RestreamView
}

type RestreamExecutor interface {
	Start() error
	Launch(string, func(context.Context) (media.Process, error), func(bool)) error
	Stop(string) error
	Close()
}

type RestreamUseCase struct {
	sources          ChannelRepository
	restreamer       Restreamer
	targets          map[string]string
	executor         RestreamExecutor
	ops              sync.Mutex
	mu               sync.Mutex
	relays           map[string]*relay
	closing, started bool
	closeOnce        sync.Once
}

func NewRestreamUseCase(
	sources ChannelRepository,
	restreamer Restreamer,
	targets map[string]string,
	executor RestreamExecutor,
) *RestreamUseCase {
	copyTargets := map[string]string{}
	for k, v := range targets {
		copyTargets[k] = v
	}
	return &RestreamUseCase{
		sources:    sources,
		restreamer: restreamer,
		targets:    copyTargets,
		executor:   executor,
		relays:     map[string]*relay{},
	}
}

func (u *RestreamUseCase) Start() error {
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closing {
		return apperror.ErrUnavailable
	}
	if u.started {
		return apperror.ErrConflict
	}
	if err := u.executor.Start(); err != nil {
		return err
	}
	u.started = true
	return nil
}

func (u *RestreamUseCase) StartRestream(id, target string) error {
	u.ops.Lock()
	defer u.ops.Unlock()
	source, e := u.sources.HubRTSP(id)
	if e != nil {
		return e
	}
	dest, ok := u.targets[target]
	if !ok {
		return apperror.ErrNotFound
	}
	u.mu.Lock()
	if u.closing {
		u.mu.Unlock()
		return apperror.ErrUnavailable
	}
	if _, ok = u.relays[target]; ok {
		u.mu.Unlock()
		return apperror.ErrConflict
	}
	r := &relay{view: model.RestreamView{Target: target, Channel: id, State: "CONNECTING"}}
	u.relays[target] = r
	u.mu.Unlock()
	if e := u.executor.Launch(target, func(ctx context.Context) (media.Process, error) {
		return u.restreamer.Restream(ctx, source, dest)
	}, func(running bool) {
		u.observe(r, running)
	}); e != nil {
		u.mu.Lock()
		delete(u.relays, target)
		u.mu.Unlock()
		return e
	}
	return nil
}

func (u *RestreamUseCase) observe(r *relay, running bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if running {
		r.view.State = "RUNNING"
	} else {
		r.view.State = "RETRYING"
	}
}

func (u *RestreamUseCase) StopRestream(target string) error {
	u.ops.Lock()
	defer u.ops.Unlock()
	u.mu.Lock()
	r, ok := u.relays[target]
	if !ok {
		u.mu.Unlock()
		return apperror.ErrNotFound
	}
	u.mu.Unlock()
	if e := u.executor.Stop(target); e != nil {
		return e
	}
	u.mu.Lock()
	if u.relays[target] == r {
		delete(u.relays, target)
	}
	u.mu.Unlock()
	return nil
}

func (u *RestreamUseCase) Restreams() []model.RestreamView {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := []model.RestreamView{}
	for _, r := range u.relays {
		out = append(out, r.view)
	}
	return out
}

func (u *RestreamUseCase) Close() {
	u.closeOnce.Do(func() {
		u.ops.Lock()
		defer u.ops.Unlock()
		u.mu.Lock()
		u.closing = true
		u.mu.Unlock()
		u.executor.Close()
	})
}
