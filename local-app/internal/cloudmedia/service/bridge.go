// Package service implements durable cloud upload without transferring ownership
// of recording or upload workers to the desktop process.
package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	cloudModel "kicau.local/edge/internal/cloudmedia/model"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/infra/storage/credentials"
	"kicau.local/edge/internal/recording/model"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

type Pairing = cloudModel.Pairing
type Mapping = cloudModel.Mapping
type Ingestion = cloudModel.Ingestion
type Progress = cloudModel.Progress
type Bridge struct {
	mu     sync.Mutex
	dir    string
	client *http.Client
	active map[string]bool
}

func New(dir string) (*Bridge, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	return &Bridge{dir: dir, client: &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, active: map[string]bool{}}, nil
}
func validBase(s string) bool {
	u, e := url.Parse(s)
	if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()))
}
func (b *Bridge) save(name string, v any, secret bool) error {
	data, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if secret {
		data, e = (credentials.Protector{}).Protect(data)
		if e != nil {
			return e
		}
	}
	f, e := os.CreateTemp(b.dir, ".pending-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(b.dir, name))
}
func (b *Bridge) load(name string, v any, secret bool) error {
	data, e := os.ReadFile(filepath.Join(b.dir, name))
	if e != nil {
		return e
	}
	if secret {
		data, e = (credentials.Protector{}).Unprotect(data)
		if e != nil {
			return e
		}
	}
	return json.Unmarshal(data, v)
}
func (b *Bridge) Configure(p Pairing) error {
	if !validBase(p.Base) || len(p.Token) != 64 || !safeID.MatchString(p.EngineID) || p.ArenaID == "" {
		return apperror.ErrInvalid
	}
	p.Base = strings.TrimRight(p.Base, "/")
	if !strings.HasSuffix(p.Base, "/api/v1") {
		p.Base += "/api/v1"
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var old Pairing
	if e := b.load("pairing.bin", &old, true); e == nil && old.ArenaID != p.ArenaID {
		return apperror.ErrConflict
	}
	return b.save("pairing.bin", p, true)
}
func (b *Bridge) Status() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()
	var p Pairing
	e := b.load("pairing.bin", &p, true)
	return map[string]any{"paired": e == nil, "base_url": p.Base, "engine_id": p.EngineID, "arena_id": p.ArenaID}
}
func (b *Bridge) Bind(in Ingestion) error {
	if !safeID.MatchString(in.SessionID) || in.ID == "" || len(in.Mappings) < 1 || len(in.Mappings) > 24 {
		return apperror.ErrInvalid
	}
	seen := map[string]bool{}
	for _, m := range in.Mappings {
		if !safeID.MatchString(m.ChannelID) || seen[m.ChannelID] {
			return apperror.ErrInvalid
		}
		seen[m.ChannelID] = true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var p Pairing
	if e := b.load("pairing.bin", &p, true); e != nil {
		return apperror.ErrUnavailable
	}
	if in.EngineID != p.EngineID {
		return apperror.ErrConflict
	}
	var old Ingestion
	if e := b.load("session-"+in.SessionID+".json", &old, false); e == nil {
		a, _ := json.Marshal(old)
		v, _ := json.Marshal(in)
		if !bytes.Equal(a, v) {
			return apperror.ErrConflict
		}
		return nil
	}
	return b.save("session-"+in.SessionID+".json", in, false)
}
func (b *Bridge) Progress(id string) Progress {
	if !safeID.MatchString(id) {
		return Progress{State: "FAILED"}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	var p Progress
	if e := b.load("upload-"+id+".json", &p, false); e != nil {
		p.State = "WAITING_MAPPING"
	}
	return p
}
func (b *Bridge) progress(id string, p Progress) {
	p.Updated = time.Now().UTC()
	b.mu.Lock()
	defer b.mu.Unlock()
	_ = b.save("upload-"+id+".json", p, false)
}
func (b *Bridge) Retry(id string) error {
	if !safeID.MatchString(id) {
		return apperror.ErrInvalid
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.active[id] {
		return apperror.ErrConflict
	}
	var p Progress
	_ = b.load("upload-"+id+".json", &p, false)
	p.Error = ""
	if p.State != "UPLOADED" {
		p.State = "QUEUED"
	}
	return b.save("upload-"+id+".json", p, false)
}
func (b *Bridge) Put(context.Context, string, string) error {
	return errors.New("recording metadata required")
}
func (b *Bridge) call(c context.Context, p Pairing, method, path string, body any, out any) error {
	var raw io.Reader
	if body != nil {
		v, e := json.Marshal(body)
		if e != nil {
			return e
		}
		raw = bytes.NewReader(v)
	}
	req, e := http.NewRequestWithContext(c, method, p.Base+"/engine/media"+path, raw)
	if e != nil {
		return apperror.ErrInvalid
	}
	req.Header.Set("Authorization", "Bearer "+p.Token)
	req.Header.Set("Accept", "application/json")
	if raw != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := b.client.Do(req)
	if e != nil {
		return apperror.ErrUnavailable
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || res.StatusCode == 403 {
		return apperror.ErrForbidden
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return apperror.ErrUnavailable
	}
	var env struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if e = json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&env); e != nil || !env.Success {
		return apperror.ErrUnavailable
	}
	return json.Unmarshal(env.Data, out)
}

type remotePart struct {
	Number int32  `json:"number"`
	SHA256 string `json:"sha256"`
}
type remote struct {
	ObjectKey string       `json:"object_key"`
	ID        string       `json:"id"`
	State     string       `json:"state"`
	PartSize  int64        `json:"part_size"`
	Parts     []remotePart `json:"parts"`
}

func (b *Bridge) PutRecording(c context.Context, r model.Recording) (err error) {
	if !safeID.MatchString(r.ID) || !safeID.MatchString(r.RoundID) {
		return apperror.ErrInvalid
	}
	b.mu.Lock()
	if b.active[r.ID] {
		b.mu.Unlock()
		return apperror.ErrConflict
	}
	b.active[r.ID] = true
	var p Pairing
	pe := b.load("pairing.bin", &p, true)
	var mapping Ingestion
	me := b.load("session-"+r.RoundID+".json", &mapping, false)
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.active, r.ID); b.mu.Unlock() }()
	progress := b.Progress(r.ID)
	failureMessage := "Upload belum selesai; file lokal dipertahankan"
	defer func() {
		if err != nil {
			progress.State = "RETRYING"
			if errors.Is(err, apperror.ErrForbidden) {
				progress.State = "ACCESS_DENIED"
			}
			if errors.Is(err, model.ErrNotMapped) {
				progress.State = "WAITING_MAPPING"
			}
			progress.Error = failureMessage
			b.progress(r.ID, progress)
		}
	}()
	if pe != nil || me != nil {
		return model.ErrNotMapped
	}
	found := false
	for _, m := range mapping.Mappings {
		found = found || m.ChannelID == r.ChannelID
	}
	if !found {
		return model.ErrNotMapped
	}
	ext := strings.ToLower(filepath.Ext(r.File))
	if ext != ".mkv" && ext != ".mp4" {
		return apperror.ErrInvalid
	}
	container := strings.TrimPrefix(ext, ".")
	f, e := os.Open(r.File)
	if e != nil {
		return e
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || stat.Size() == 0 {
		return apperror.ErrInvalid
	}
	oldSize := progress.Size
	progress.Size = stat.Size()
	hash := progress.SHA256
	if hash == "" || oldSize != stat.Size() || !progress.Modified.Equal(stat.ModTime()) {
		progress.State = "HASHING"
		progress.Bytes = 0
		b.progress(r.ID, progress)
		h := sha256.New()
		if _, e = copyContext(c, h, f); e != nil {
			return e
		}
		hash = hex.EncodeToString(h.Sum(nil))
		progress.SHA256 = hash
		progress.Modified = stat.ModTime()
		progress.State = "QUEUED"
		b.progress(r.ID, progress)
	}
	if _, e = f.Seek(0, io.SeekStart); e != nil {
		return e
	}
	var upload remote
	payload := map[string]any{"container": container, "session_id": r.RoundID, "channel_id": r.ChannelID, "recording_id": r.ID, "size_bytes": stat.Size(), "sha256": hash, "started_at": r.StartedAt, "ended_at": r.EndedAt}
	if e = b.call(c, p, "POST", "/uploads", payload, &upload); e != nil {
		return e
	}
	if !strings.HasSuffix(upload.ObjectKey, ext) {
		failureMessage = "Cloud belum mendukung " + strings.ToUpper(container) + "; perbarui cloud-app. File lokal dipertahankan"
		return apperror.ErrUnavailable
	}
	progress.UploadID = upload.ID
	if upload.State == "UPLOADED" {
		progress.State = "UPLOADED"
		progress.Bytes = stat.Size()
		progress.Error = ""
		b.progress(r.ID, progress)
		return nil
	}
	if upload.PartSize < 5<<20 || upload.PartSize > 64<<20 || !safeID.MatchString(upload.ID) {
		return apperror.ErrInvalid
	}
	done := map[int32]string{}
	for _, v := range upload.Parts {
		done[v.Number] = v.SHA256
	}
	full := sha256.New()
	buf := make([]byte, upload.PartSize)
	progress.Bytes = 0
	progress.State = "UPLOADING"
	progress.Error = ""
	for number := int32(1); ; number++ {
		if e = c.Err(); e != nil {
			return e
		}
		n, e := io.ReadFull(f, buf)
		if e != nil && e != io.EOF && e != io.ErrUnexpectedEOF {
			return e
		}
		if n == 0 {
			break
		}
		data := buf[:n]
		full.Write(data)
		sum := sha256.Sum256(data)
		checksum := base64.StdEncoding.EncodeToString(sum[:])
		if done[number] != checksum {
			var signed struct {
				URL     string            `json:"url"`
				Headers map[string]string `json:"headers"`
			}
			if e = b.call(c, p, "POST", "/uploads/"+upload.ID+"/parts", map[string]any{"number": number, "sha256": checksum}, &signed); e != nil {
				return e
			}
			if !validSigned(signed.URL) {
				return apperror.ErrInvalid
			}
			req, e := http.NewRequestWithContext(c, "PUT", signed.URL, bytes.NewReader(data))
			if e != nil {
				return apperror.ErrInvalid
			}
			for k, v := range signed.Headers {
				req.Header.Set(k, v)
			}
			req.ContentLength = int64(n)
			res, e := b.client.Do(req)
			if e != nil {
				return apperror.ErrUnavailable
			}
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
			res.Body.Close()
			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return fmt.Errorf("storage upload rejected (%d)", res.StatusCode)
			}
		}
		progress.Bytes += int64(n)
		b.progress(r.ID, progress)
	}
	if hex.EncodeToString(full.Sum(nil)) != hash {
		return apperror.ErrConflict
	}
	progress.State = "SYNCING"
	b.progress(r.ID, progress)
	if e = b.call(c, p, "POST", "/uploads/"+upload.ID+"/complete", map[string]any{}, &upload); e != nil {
		return e
	}
	if upload.State != "UPLOADED" {
		return apperror.ErrUnavailable
	}
	progress.State = "UPLOADED"
	progress.Error = ""
	b.progress(r.ID, progress)
	return nil
}

func validSigned(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil {
		return false
	}
	u.RawQuery = ""
	return validBase(u.String())
}

func copyContext(c context.Context, w io.Writer, r io.Reader) (int64, error) {
	buf := make([]byte, 1<<20)
	var total int64
	for {
		if e := c.Err(); e != nil {
			return total, e
		}
		n, e := r.Read(buf)
		if n > 0 {
			written, we := w.Write(buf[:n])
			total += int64(written)
			if we != nil {
				return total, we
			}
			if written != n {
				return total, io.ErrShortWrite
			}
		}
		if e == io.EOF {
			return total, nil
		}
		if e != nil {
			return total, e
		}
	}
}
