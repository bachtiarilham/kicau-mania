// Package repository stores engine command journals and pending media metadata.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/cloud/model"
	"kicau.local/edge/internal/core/identifier"
	"kicau.local/edge/internal/infra/storage/filesystem"
)

type Files struct {
	dir string
	mu  sync.Mutex
}
type commandEntry struct {
	Command      model.Command        `json:"command"`
	Result       *model.CommandResult `json:"result,omitempty"`
	Acknowledged bool                 `json:"acknowledged"`
}
type updateEntry struct {
	Update       model.MediaUpdate `json:"update"`
	Acknowledged bool              `json:"acknowledged"`
}

func New(dir string) (*Files, error) {
	for _, name := range []string{"commands", "media-updates"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0700); err != nil {
			return nil, err
		}
	}
	return &Files{dir: dir}, nil
}
func (f *Files) read(folder, id string, out any) error {
	if !identifier.Valid(id) {
		return apperror.ErrInvalid
	}
	data, err := os.ReadFile(filepath.Join(f.dir, folder, id+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return apperror.ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
func (f *Files) write(folder, id string, value any) error {
	if !identifier.Valid(id) {
		return apperror.ErrInvalid
	}
	return filesystem.WriteJSON(filepath.Join(f.dir, folder, id+".json"), value)
}
func (f *Files) ids(folder string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(f.dir, folder))
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	sort.Strings(ids)
	return ids, nil
}
func (f *Files) Command(ctx context.Context, id string) (model.Command, *model.CommandResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return model.Command{}, nil, err
	}
	var entry commandEntry
	err := f.read("commands", id, &entry)
	return entry.Command, entry.Result, err
}
func (f *Files) SaveCommand(ctx context.Context, c model.Command) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	var old commandEntry
	err := f.read("commands", c.ID, &old)
	if err == nil {
		a, _ := json.Marshal(old.Command)
		b, _ := json.Marshal(c)
		if string(a) != string(b) {
			return fmt.Errorf("command ID reused")
		}
		return nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return err
	}
	return f.write("commands", c.ID, commandEntry{Command: c})
}
func (f *Files) CompleteCommand(ctx context.Context, r model.CommandResult) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	var entry commandEntry
	if err := f.read("commands", r.ID, &entry); err != nil {
		return err
	}
	if entry.Result != nil {
		return nil
	}
	entry.Result = &r
	return f.write("commands", r.ID, entry)
}
func (f *Files) CommandResults(ctx context.Context) ([]model.CommandResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []model.CommandResult{}
	ids, err := f.ids("commands")
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var entry commandEntry
		if err := f.read("commands", id, &entry); err != nil {
			return nil, err
		}
		if entry.Result != nil && !entry.Acknowledged {
			out = append(out, *entry.Result)
			if len(out) == 100 {
				break
			}
		}
	}
	return out, nil
}
func (f *Files) AcknowledgeCommands(ctx context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		var entry commandEntry
		if err := f.read("commands", id, &entry); err != nil {
			return err
		}
		if entry.Result == nil {
			return apperror.ErrConflict
		}
		entry.Acknowledged = true
		if err := f.write("commands", id, entry); err != nil {
			return err
		}
	}
	return nil
}
func (f *Files) Enqueue(ctx context.Context, u model.MediaUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	var old updateEntry
	err := f.read("media-updates", u.ID, &old)
	if err == nil {
		return nil
	}
	if !errors.Is(err, apperror.ErrNotFound) {
		return err
	}
	return f.write("media-updates", u.ID, updateEntry{Update: u})
}
func (f *Files) Pending(ctx context.Context) ([]model.MediaUpdate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids, err := f.ids("media-updates")
	if err != nil {
		return nil, err
	}
	out := []model.MediaUpdate{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var entry updateEntry
		if err := f.read("media-updates", id, &entry); err != nil {
			return nil, err
		}
		if !entry.Acknowledged {
			out = append(out, entry.Update)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RecordingID == out[j].RecordingID {
			return out[i].Version < out[j].Version
		}
		return out[i].RecordingID < out[j].RecordingID
	})
	if len(out) > 100 {
		out = out[:100]
	}
	return out, nil
}
func (f *Files) Acknowledge(ctx context.Context, ids []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		var entry updateEntry
		if err := f.read("media-updates", id, &entry); err != nil {
			return err
		}
		entry.Acknowledged = true
		if err := f.write("media-updates", id, entry); err != nil {
			return err
		}
	}
	return nil
}
