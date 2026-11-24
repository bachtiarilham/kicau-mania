package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"kicau.local/edge/config"
	cameraDelivery "kicau.local/edge/internal/camera/delivery"
	cameraRepo "kicau.local/edge/internal/camera/repository"
	cameraUC "kicau.local/edge/internal/camera/usecase"
	recordingDelivery "kicau.local/edge/internal/recording/delivery"
	recordingModel "kicau.local/edge/internal/recording/model"
	recordingRepo "kicau.local/edge/internal/recording/repository"
	recordingUC "kicau.local/edge/internal/recording/usecase"
	recordingWorker "kicau.local/edge/internal/recording/worker"
	streamingDelivery "kicau.local/edge/internal/streaming/delivery"
	streamingService "kicau.local/edge/internal/streaming/service"
	streamingUC "kicau.local/edge/internal/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/streaming/worker"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"kicau.local/edge/internal/mediaengine/media"
	"kicau.local/edge/internal/mediaengine/media/mediamtx"
)

type observed struct {
	kind, source string
	done         <-chan struct{}
}
type fakeProcess struct {
	done chan struct{}
	once sync.Once
}

func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) Err() error            { <-p.done; return nil }
func (p *fakeProcess) Stop()                 { p.once.Do(func() { close(p.done) }) }

type fakeMedia struct {
	events chan observed
	wg     sync.WaitGroup
}

func (f *fakeMedia) start(ctx context.Context, kind, source string) media.Process {
	p := &fakeProcess{done: make(chan struct{})}
	f.events <- observed{kind, source, p.done}
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		select {
		case <-ctx.Done():
			p.Stop()
		case <-p.done:
		}
	}()
	return p
}
func (f *fakeMedia) Restream(ctx context.Context, source, _ string) (media.Process, error) {
	return f.start(ctx, "streaming", source), nil
}
func (f *fakeMedia) RunMedia(ctx context.Context, source, file string, ready func(), write func([]byte)) error {
	if err := os.WriteFile(file, []byte("media fixture"), 0600); err != nil {
		return err
	}
	f.events <- observed{"pipeline", source, ctx.Done()}
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			ready()
			write(make([]byte, 6400))
		}
	}
}
func (f *fakeMedia) Finalize(_ context.Context, parts []string, file string) error {
	if len(parts) == 0 {
		return os.ErrNotExist
	}
	return os.WriteFile(file, []byte("validated fixture"), 0600)
}

type testDisk struct{ filesystem.Disk }

type testContainer struct {
	Camera    *cameraUC.CameraUseCase
	Audio     *streamingUC.AudioUseCase
	Streaming *streamingUC.RestreamUseCase
	Saving    *recordingUC.SavingUseCase
	Sessions  *recordingUC.SessionUseCase
	Handlers  Handlers
}

func (testDisk) Free(string) (uint64, error) { return 1 << 40, nil }

func fixture(t *testing.T) (*testContainer, config.Config, *fakeMedia) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Token: strings.Repeat("a", 32), Origins: []string{"http://arena.test"}}
	catalog, e := cameraRepo.New([]cameraRepo.Input{{ID: "lapak01", Source: "rtsp://user:secret@192.0.2.1/a"}}, "rtsp://127.0.0.1:8554")
	if e != nil {
		t.Fatal(e)
	}
	mediaServer := mediamtx.New(mediamtx.Options{})
	repo, e := recordingRepo.NewFiles(filepath.Join(dir, "queue"))
	if e != nil {
		t.Fatal(e)
	}
	media := &fakeMedia{events: make(chan observed, 10)}
	v := cameraUC.NewCameraUseCase(catalog, catalog)
	a := streamingUC.NewAudioUseCase(streamingUC.AudioOptions{SampleRate: 16000, Threshold: -35}, catalog, streamingService.NewBroadcaster(), nil, streamingWorker.NewAudioWorker())
	s := streamingUC.NewRestreamUseCase(catalog, media, map[string]string{"youtube": "rtmps://example.test/live/key"}, streamingWorker.NewRestreamWorker())
	save := recordingUC.NewSavingUseCase(repo, nil, recordingWorker.NewUploadWorker())
	journal, e := recordingRepo.NewJournal(filepath.Join(dir, "sessions"))
	if e != nil {
		t.Fatal(e)
	}
	sessions := recordingUC.NewSessionUseCase(recordingUC.SessionOptions{DataDir: dir, MaxDuration: time.Hour, MinFreeBytes: 100, Targets: map[string]string{"youtube": "configured"}}, journal, repo, catalog, media, media, a, s, testDisk{}, recordingWorker.NewSessionWorker(), mediaServer)
	streamHandler := streamingDelivery.NewHandler(s, a, cfg.Origins)
	sessions.BindConnections(nil, streamHandler)
	c := &testContainer{Camera: v, Audio: a, Streaming: s, Saving: save, Sessions: sessions, Handlers: Handlers{Camera: cameraDelivery.NewHandler(v, nil), Recording: recordingDelivery.NewHandler(sessions, save), Streaming: streamHandler}}

	t.Cleanup(func() {
		sessions.Close()
		c.Handlers.Streaming.Close()
		save.Close()
		s.Close()
		a.Close()
		media.wg.Wait()
	})
	for _, start := range []func() error{save.Start, a.Start, s.Start, sessions.Start} {
		if e = start(); e != nil {
			t.Fatal(e)
		}
	}
	return c, cfg, media
}
func waitStopped(t *testing.T, c *testContainer, id string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		s, err := c.Sessions.Get(context.Background(), id)
		if err == nil && s.State == "STOPPED" {
			return
		}
		time.Sleep(time.Millisecond * 10)
	}
	t.Fatal("session did not finalize")
}
func TestStopSessionStopsAllMedia(t *testing.T) {
	c, _, media := fixture(t)
	_, err := c.Sessions.StartSession(context.Background(), recordingModel.StartRequest{ID: "session1", Channels: []string{"lapak01"}, Restreams: []recordingModel.Restream{{ChannelID: "lapak01", Target: "youtube"}}})
	if err != nil {
		t.Fatal(err)
	}
	pcm, cancel, err := c.Audio.SubscribeAudio("lapak01")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	events := map[string]observed{}
	for len(events) < 2 {
		select {
		case event := <-media.events:
			events[event.kind] = event
		case <-time.After(2 * time.Second):
			t.Fatal("media not started")
		}
	}
	select {
	case <-pcm:
	case <-time.After(time.Second):
		t.Fatal("PCM missing")
	}
	if _, err = c.Sessions.StopSession(context.Background(), "session1"); err != nil {
		t.Fatal(err)
	}
	waitStopped(t, c, "session1")
	for kind, event := range events {
		select {
		case <-event.done:
		default:
			t.Fatal("media survived stop", kind)
		}
	}
	if _, _, err = c.Audio.SubscribeAudio("lapak01"); err == nil {
		t.Fatal("audio admitted outside session")
	}
	records, _, err := c.Saving.Recordings(context.Background(), "", 25)
	if err != nil || len(records) != 1 || records[0].State != "READY" {
		t.Fatal(records, err)
	}
}

func TestExistingHTTPContracts(t *testing.T) {
	c, cfg, _ := fixture(t)
	router := Router(cfg, c.Handlers)
	request := func(method, path, body string, auth bool) (int, map[string]json.RawMessage) {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		if auth {
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		var result map[string]json.RawMessage
		if w.Body.Len() > 0 {
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
				t.Fatal(w.Body.String(), e)
			}
		}
		return w.Code, result
	}
	if status, _ := request("GET", "/healthz", "", false); status != 200 {
		t.Fatal(status)
	}
	if status, _ := request("GET", "/api/v1/channels", "", false); status != 401 {
		t.Fatal(status)
	}
	status, body := request("GET", "/api/v1/channels", "", true)
	if status != 200 {
		t.Fatal(status)
	}
	var channels []map[string]string
	if e := json.Unmarshal(body["data"], &channels); e != nil {
		t.Fatal(e)
	}
	if len(channels) != 1 || len(channels[0]) != 3 || channels[0]["id"] != "lapak01" || channels[0]["rtsp_url"] != "rtsp://127.0.0.1:8554/lapak01" || channels[0]["audio_path"] != "/api/v1/channels/lapak01/audio" {
		t.Fatal(channels)
	}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/v1/media/sessions", `{`, 400},
		{"POST", "/api/v1/media/sessions", `{"id":"r1","channels":["lapak01","lapak01"]}`, 400},
		{"POST", "/api/v1/media/sessions", `{"id":"r1","channels":["lapak01"]}`, 202},
		{"POST", "/api/v1/media/sessions", `{"id":"r2","channels":["lapak01"]}`, 409},
		{"GET", "/api/v1/recordings?limit=abc", "", 400},
		{"GET", "/api/v1/recordings?limit=0", "", 400},
		{"GET", "/api/v1/media/sessions/r1", "", 200},
		{"POST", "/api/v1/media/sessions/r1/stop", "", 202},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"missing"}`, 405},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"youtube"}`, 405},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"youtube"}`, 405},
		{"GET", "/api/v1/restreams", "", 200},
		{"DELETE", "/api/v1/restreams/youtube", "", 405},
	} {
		if got, _ := request(tc.method, tc.path, tc.body, true); got != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, got, tc.want)
		}
	}
	waitStopped(t, c, "r1")
	status, body = request("GET", "/api/v1/recordings?limit=25", "", true)
	if status != 200 {
		t.Fatal(status)
	}
	var page struct {
		Items      []map[string]json.RawMessage `json:"items"`
		NextCursor string                       `json:"next_cursor"`
		HasMore    bool                         `json:"has_more"`
	}
	if e := json.Unmarshal(body["data"], &page); e != nil {
		t.Fatal(e)
	}
	recordings := page.Items
	if len(recordings) != 1 || len(recordings[0]) != 10 || string(recordings[0]["state"]) != `"READY"` || recordings[0]["file"] != nil || page.NextCursor != "" || page.HasMore || string(body["success"]) != "true" {
		t.Fatal(string(body["data"]))
	}
	preflight := httptest.NewRequest("OPTIONS", "/api/v1/media/sessions", nil)
	preflight.Header.Set("Origin", "http://arena.test")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, preflight)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://arena.test" {
		t.Fatal("preflight changed", w.Code, w.Header())
	}
}
func TestAudioWebSocketWireContracts(t *testing.T) {
	c, cfg, _ := fixture(t)
	if _, err := c.Sessions.StartSession(context.Background(), recordingModel.StartRequest{ID: "websocket-session", Channels: []string{"lapak01"}}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Router(cfg, c.Handlers))
	defer srv.Close()
	for _, tc := range []struct {
		path string
		kind int
	}{{"/api/v1/channels/lapak01/audio", websocket.BinaryMessage}, {"/api/v1/metrics", websocket.TextMessage}} {
		dialer := websocket.Dialer{Subprotocols: []string{"edge-v1"}}
		headers := http.Header{"Authorization": []string{"Bearer " + cfg.Token}, "Origin": []string{"http://arena.test"}}
		conn, _, e := dialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+tc.path, headers)
		if e != nil {
			t.Fatal(e)
		}
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		kind, b, e := conn.ReadMessage()
		conn.Close()
		if e != nil {
			t.Fatal(e)
		}
		if kind != tc.kind || conn.Subprotocol() != "edge-v1" {
			t.Fatal("WebSocket protocol changed")
		}
		if kind == websocket.BinaryMessage {
			if len(b) != 6400 {
				t.Fatal("PCM window changed", len(b))
			}
		} else {
			var metric map[string]json.RawMessage
			if e = json.Unmarshal(b, &metric); e != nil {
				t.Fatal(e)
			}
			if len(metric) != 6 || string(metric["channel_id"]) != `"lapak01"` || string(metric["window_ms"]) != "200" {
				t.Fatal(string(b))
			}
		}
	}
}
