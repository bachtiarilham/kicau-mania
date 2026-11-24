package usecase

import (
	"context"
	"sync"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/recording/model"
)

type RecordingRepository interface {
	Save(context.Context, model.Recording) error
	List(context.Context) ([]model.Recording, error)
}

type Storage interface {
	Put(context.Context, string, string) error
}

type UploadExecutor interface {
	Start() error
	StartUploadLoop(func(context.Context)) error
	UploadWithTimeout(context.Context, func(context.Context) error) error
	Close()
}

type SavingUseCase struct {
	repo             RecordingRepository
	storage          Storage
	executor         UploadExecutor
	mu               sync.Mutex
	started, closing bool
}

func NewSavingUseCase(repo RecordingRepository, storage Storage, executor UploadExecutor) *SavingUseCase {
	return &SavingUseCase{repo: repo, storage: storage, executor: executor}
}

func (u *SavingUseCase) Start() error {
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
	if u.storage != nil {
		if err := u.executor.StartUploadLoop(u.uploadPending); err != nil {
			u.closing = true
			u.executor.Close()
			return err
		}
	}
	u.started = true
	return nil
}

func (u *SavingUseCase) Close() {
	u.mu.Lock()
	u.closing = true
	u.mu.Unlock()
	u.executor.Close()
}
