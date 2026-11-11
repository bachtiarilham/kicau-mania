// Package worker owns cancellable setup jobs and short-lived credential references.
package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/setup/model"
)

type entry struct {
	job    model.Job
	cancel context.CancelFunc
}
type secret struct {
	value string
	at    time.Time
}
type jobKey struct{}
type Jobs struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	jobs    map[string]*entry
	secrets map[string]secret
	active  int
	closed  bool
}

func New(ctx context.Context) *Jobs {
	ctx, c := context.WithCancel(ctx)
	return &Jobs{ctx: ctx, cancel: c, jobs: map[string]*entry{}, secrets: map[string]secret{}}
}
func random() string {
	var b [24]byte
	if _, e := rand.Read(b[:]); e != nil {
		panic("random source unavailable")
	}
	return hex.EncodeToString(b[:])
}
func (j *Jobs) prune() {
	for k, v := range j.jobs {
		if v.job.State != "RUNNING" && time.Since(v.job.CreatedAt) > 15*time.Minute {
			delete(j.jobs, k)
		}
	}
	for k, v := range j.secrets {
		if time.Since(v.at) > 15*time.Minute {
			delete(j.secrets, k)
		}
	}
}
func (j *Jobs) Submit(label string, fn func(context.Context) (any, error)) (model.Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.prune()
	if j.closed || j.active >= 2 || len(j.jobs) >= 64 {
		return model.Job{}, apperror.ErrConflict
	}
	ctx, cancel := context.WithTimeout(j.ctx, 5*time.Minute)
	v := &entry{job: model.Job{ID: random(), State: "RUNNING", Progress: label, CreatedAt: time.Now().UTC()}, cancel: cancel}
	ctx = context.WithValue(ctx, jobKey{}, v.job.ID)
	j.jobs[v.job.ID] = v
	j.active++
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		defer cancel()
		result, e := fn(ctx)
		j.mu.Lock()
		defer j.mu.Unlock()
		j.active--
		v.job.Progress = ""
		v.job.State = "SUCCEEDED"
		v.job.Result = result
		if e != nil {
			v.job.State = "FAILED"
			v.job.Result = nil
			v.job.Code = "SETUP_FAILED"
			v.job.Message = "Pemeriksaan gagal. Periksa alamat, profil, kredensial, dan koneksi perangkat."
			var p *apperror.Message
			if errors.As(e, &p) {
				v.job.Message = p.Text
				if p.Code != "" {
					v.job.Code = p.Code
				}
			}
			var d *model.Failure
			if errors.As(e, &d) {
				v.job.Code = d.Code
				v.job.Message = d.Message
			}
			if errors.Is(e, context.Canceled) {
				v.job.Code = "CANCELLED"
				v.job.Message = "Pekerjaan dibatalkan"
			}
			if errors.Is(e, context.DeadlineExceeded) {
				v.job.Code = "TIMEOUT"
				v.job.Message = "Perangkat tidak merespons dalam batas waktu"
			}
			v.job.Advice = model.Advice(v.job.Code)
		}
	}()
	return v.job, nil
}
func (j *Jobs) Report(ctx context.Context, message string) {
	id, _ := ctx.Value(jobKey{}).(string)
	j.mu.Lock()
	defer j.mu.Unlock()
	if v := j.jobs[id]; v != nil && v.job.State == "RUNNING" {
		v.job.Progress = message
	}
}
func (j *Jobs) Get(id string) (model.Job, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.prune()
	v := j.jobs[id]
	if v == nil {
		return model.Job{}, apperror.ErrNotFound
	}
	return v.job, nil
}
func (j *Jobs) Cancel(id string) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := j.jobs[id]
	if v == nil {
		return apperror.ErrNotFound
	}
	v.cancel()
	return nil
}
func (j *Jobs) Remember(value string) string {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.prune()
	id := random()
	j.secrets[id] = secret{value, time.Now()}
	return id
}
func (j *Jobs) Resolve(id string) (string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.prune()
	v, ok := j.secrets[id]
	if !ok {
		return "", apperror.Public(apperror.ErrInvalid, "Profil kedaluwarsa; baca ulang profil perangkat")
	}
	return v.value, nil
}
func (j *Jobs) Close() { j.mu.Lock(); j.closed = true; j.cancel(); j.mu.Unlock(); j.wg.Wait() }
