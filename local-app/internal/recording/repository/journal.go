package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"kicau.local/edge/internal/recording/model"
)

type Journal struct{ dir string }

func NewJournal(dir string) (*Journal, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Journal{dir: dir}, nil
}

func (j *Journal) Save(ctx context.Context, s model.Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !identifier.Valid(s.ID) {
		return apperror.ErrInvalid
	}
	return filesystem.WriteJSON(filepath.Join(j.dir, s.ID+".json"), s)
}

func (j *Journal) Get(ctx context.Context, id string) (model.Session, error) {
	var s model.Session
	if err := ctx.Err(); err != nil {
		return s, err
	}
	if !identifier.Valid(id) {
		return s, apperror.ErrInvalid
	}
	b, err := os.ReadFile(filepath.Join(j.dir, id+".json"))
	if os.IsNotExist(err) {
		return s, apperror.ErrNotFound
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	if err == nil && s.ID != id {
		err = apperror.ErrInvalid
	}
	return s, err
}

func (j *Journal) List(ctx context.Context) ([]model.Session, error) {
	entries, err := os.ReadDir(j.dir)
	if err != nil {
		return nil, err
	}
	out := []model.Session{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		s, err := j.Get(ctx, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}
