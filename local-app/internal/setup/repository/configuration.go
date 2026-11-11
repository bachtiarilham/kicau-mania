// Package repository persists encrypted settings by flush and atomic rename.
package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/setup/model"
)

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}
type Configuration struct {
	path      string
	protector Protector
}

func New(path string, p Protector) *Configuration { return &Configuration{path, p} }
func (r *Configuration) Load(ctx context.Context) (model.Configuration, error) {
	var c model.Configuration
	if e := ctx.Err(); e != nil {
		return c, e
	}
	b, e := os.ReadFile(r.path)
	if os.IsNotExist(e) {
		return c, apperror.ErrNotFound
	}
	if e != nil {
		return c, e
	}
	var envelope struct {
		Version   int    `json:"version"`
		Protected []byte `json:"protected"`
	}
	if json.Unmarshal(b, &envelope) != nil || envelope.Version != 1 {
		return c, apperror.ErrInvalid
	}
	b, e = r.protector.Unprotect(envelope.Protected)
	if e != nil {
		return c, apperror.Public(apperror.ErrUnavailable, "Konfigurasi tidak dapat dibuka oleh akun Windows ini; gunakan akun yang menyimpannya")
	}
	if json.Unmarshal(b, &c) != nil || c.Version != 1 {
		return c, apperror.ErrInvalid
	}
	return c, nil
}
func (r *Configuration) Save(ctx context.Context, c model.Configuration) error {
	if e := ctx.Err(); e != nil {
		return e
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return e
	}
	raw, e = r.protector.Protect(raw)
	if e != nil {
		return e
	}
	b, e := json.Marshal(struct {
		Version   int    `json:"version"`
		Protected []byte `json:"protected"`
	}{1, raw})
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(r.path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(r.path), ".setup-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(name, r.path)
}
