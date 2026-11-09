package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/mediaengine/saving/model"
)

type Files struct {
	dir string
	mu  sync.Mutex
}

func NewFiles(dir string) (*Files, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	return &Files{dir: dir}, nil
}

func (r *Files) Save(ctx context.Context, v model.Recording) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !identifier.Valid(v.ID) {
		return fmt.Errorf("unsafe manifest name")
	}
	previous, readErr := os.ReadFile(filepath.Join(r.dir, v.ID+".json"))
	if readErr == nil {
		var old model.Recording
		if err := json.Unmarshal(previous, &old); err != nil {
			return err
		}
		v.Version = old.Version
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	v.Version++
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(r.dir, ".manifest-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, filepath.Join(r.dir, v.ID+".json"))
}

func (r *Files) List(ctx context.Context) ([]model.Recording, error) {
	entries, e := os.ReadDir(r.dir)
	if e != nil {
		return nil, e
	}
	out := []model.Recording{}
	for _, f := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if f.IsDir() || filepath.Ext(f.Name()) != ".json" {
			continue
		}
		b, e := os.ReadFile(filepath.Join(r.dir, f.Name()))
		if e != nil {
			return nil, e
		}
		var v model.Recording
		if e = json.Unmarshal(b, &v); e != nil {
			return nil, fmt.Errorf("corrupt recording manifest %s", f.Name())
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
