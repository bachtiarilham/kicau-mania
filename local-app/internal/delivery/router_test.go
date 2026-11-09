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
	channelDelivery "kicau.local/edge/internal/delivery/rest/channel"
	savingDelivery "kicau.local/edge/internal/delivery/rest/saving"
	streamingDelivery "kicau.local/edge/internal/delivery/rest/streaming"
	websocketDelivery "kicau.local/edge/internal/delivery/websocket"
	"kicau.local/edge/internal/infra/storage/filesystem"
	audioService "kicau.local/edge/internal/mediaengine/audio/service"
	audioUC "kicau.local/edge/internal/mediaengine/audio/usecase"
	audioWorker "kicau.local/edge/internal/mediaengine/audio/worker"
	channelRepo "kicau.local/edge/internal/mediaengine/channel/repository"
	channelUC "kicau.local/edge/internal/mediaengine/channel/usecase"
	"kicau.local/edge/internal/mediaengine/media"
	"kicau.local/edge/internal/mediaengine/media/mediamtx"
	savingModel "kicau.local/edge/internal/mediaengine/saving/model"
	savingRepo "kicau.local/edge/internal/mediaengine/saving/repository"
	savingUC "kicau.local/edge/internal/mediaengine/saving/usecase"
	savingWorker "kicau.local/edge/internal/mediaengine/saving/worker"
	streamingUC "kicau.local/edge/internal/mediaengine/streaming/usecase"
	streamingWorker "kicau.local/edge/internal/mediaengine/streaming/worker"
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
func (f *fakeMedia) Record(ctx context.Context, source, file string) (media.Process, error) {
	if e := os.WriteFile(file, []byte("finalized fixture"), 0600); e != nil {
		return nil, e
	}
	return f.start(ctx, "saving", source), nil
}
func (f *fakeMedia) Audio(ctx context.Context, source string, write func([]byte)) error {
	f.events <- observed{"audio", source, ctx.Done()}
	timer := time.NewTicker(10 * time.Millisecond)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			write(make([]byte, 6400))
		}
	}
}

type testDisk struct{ filesystem.Disk }

type testContainer struct {
	Channels  *channelUC.UseCase
	Audio     *audioUC.UseCase
	Streaming *streamingUC.UseCase
	Saving    *savingUC.UseCase
	Handlers  Handlers
}

func (testDisk) Free(string) (uint64, error) { return 1 << 40, nil }

func fixture(t *testing.T) (*testContainer, config.Config, *fakeMedia) {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{Token: strings.Repeat("a", 32), Origins: []string{"http://arena.test"}}
	catalog, e := channelRepo.New([]channelRepo.Input{{ID: "lapak01", Source: "rtsp://user:secret@192.0.2.1/a"}}, "rtsp://127.0.0.1:8554")
	if e != nil {
		t.Fatal(e)
	}
	mediaServer := mediamtx.New(mediamtx.Options{WebRTCBase: "http://192.0.2.2:8889"})
	repo, e := savingRepo.NewFiles(filepath.Join(dir, "queue"))
	if e != nil {
		t.Fatal(e)
	}
	media := &fakeMedia{events: make(chan observed, 10)}
	v := channelUC.New(catalog, mediaServer)
	a := audioUC.New(audioUC.Options{SampleRate: 16000, Threshold: -35}, catalog, media, audioService.NewBroadcaster(), nil, audioWorker.New())
	s := streamingUC.New(catalog, media, map[string]string{"youtube": "rtmps://example.test/live/key"}, streamingWorker.New())
	save := savingUC.New(savingUC.Options{DataDir: dir, MaxRound: time.Hour, MinFreeBytes: 100}, catalog, repo, media, nil, testDisk{}, savingWorker.New())
	c := &testContainer{Channels: v, Audio: a, Streaming: s, Saving: save, Handlers: Handlers{Channels: channelDelivery.New(v), WebSocket: websocketDelivery.New(a, cfg.Origins), Streaming: streamingDelivery.New(s), Saving: savingDelivery.New(save)}}
	t.Cleanup(func() {
		c.Handlers.WebSocket.Close()
		save.Close()
		s.Close()
		a.Close()
		media.wg.Wait()
	})
	for _, start := range []func() error{save.Start, a.Start, s.Start} {
		if e = start(); e != nil {
			t.Fatal(e)
		}
	}
	return c, cfg, media
}
func TestStopStreamingDoesNotStopOtherDomains(t *testing.T) {
	c, _, media := fixture(t)
	pcm, unsubscribe, e := c.Audio.SubscribeAudio("lapak01")
	if e != nil {
		t.Fatal(e)
	}
	defer unsubscribe()
	if _, e = c.Saving.StartSession(context.Background(), savingModel.StartSession{ID: "round1", Channels: []string{"lapak01"}}); e != nil {
		t.Fatal(e)
	}
	if e = c.Streaming.StartRestream("lapak01", "youtube"); e != nil {
		t.Fatal(e)
	}
	events := map[string]observed{}
	for len(events) < 3 {
		select {
		case event := <-media.events:
			if event.source != "rtsp://127.0.0.1:8554/lapak01" {
				t.Fatal("domain bypassed NVR source resolution", event)
			}
			events[event.kind] = event
		case <-time.After(2 * time.Second):
			t.Fatal("workers not started", events)
		}
	}
	if e = c.Streaming.StopRestream("youtube"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-events["streaming"].done:
	default:
		t.Fatal("streaming process survived stop")
	}
	for _, kind := range []string{"audio", "saving"} {
		select {
		case <-events[kind].done:
			t.Fatal("streaming stopped another domain", kind)
		default:
		}
	}
	channels := c.Channels.Channels()
	if len(channels) != 1 || channels[0].WHEP != "http://192.0.2.2:8889/api/v1/playback/lapak01/whep" {
		t.Fatal("restream changed local video metadata")
	}
	select {
	case b := <-pcm:
		if len(b) != 6400 {
			t.Fatal("audio frame changed")
		}
	case <-time.After(time.Second):
		t.Fatal("audio stopped")
	}
	if e = c.Saving.StopSession(context.Background(), "round1"); e != nil {
		t.Fatal(e)
	}
	records, _, e := c.Saving.Recordings(context.Background(), "", 25)
	if e != nil || len(records) != 1 || records[0].State != "READY" {
		t.Fatalf("saving finalization: %+v %v", records, e)
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
	if len(channels) != 1 || len(channels[0]) != 3 || channels[0]["id"] != "lapak01" || channels[0]["whep_url"] != "http://192.0.2.2:8889/api/v1/playback/lapak01/whep" || channels[0]["audio_path"] != "/api/v1/channels/lapak01/audio" {
		t.Fatal(channels)
	}
	for _, tc := range []struct {
		method, path, body string
		want               int
	}{
		{"POST", "/api/v1/media/recording-sessions", `{`, 400},
		{"POST", "/api/v1/media/recording-sessions", `{"id":"r1","channels":["lapak01","lapak01"]}`, 400},
		{"POST", "/api/v1/media/recording-sessions", `{"id":"r1","channels":["lapak01"]}`, 202},
		{"POST", "/api/v1/media/recording-sessions", `{"id":"r2","channels":["lapak01"]}`, 409},
		{"GET", "/api/v1/recordings?limit=abc", "", 400},
		{"GET", "/api/v1/recordings?limit=0", "", 400},
		{"POST", "/api/v1/media/recording-sessions/r1/stop", "", 204},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"missing"}`, 404},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"youtube"}`, 202},
		{"POST", "/api/v1/restreams", `{"channel_id":"lapak01","target":"youtube"}`, 409},
		{"GET", "/api/v1/restreams", "", 200},
		{"DELETE", "/api/v1/restreams/youtube", "", 204},
	} {
		if got, _ := request(tc.method, tc.path, tc.body, true); got != tc.want {
			t.Fatalf("%s %s: got %d want %d", tc.method, tc.path, got, tc.want)
		}
	}
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
	if len(recordings) != 1 || len(recordings[0]) != 9 || string(recordings[0]["state"]) != `"READY"` || recordings[0]["file"] != nil || page.NextCursor != "" || page.HasMore || string(body["success"]) != "true" {
		t.Fatal(string(body["data"]))
	}
	preflight := httptest.NewRequest("OPTIONS", "/api/v1/media/recording-sessions", nil)
	preflight.Header.Set("Origin", "http://arena.test")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, preflight)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "http://arena.test" {
		t.Fatal("preflight changed", w.Code, w.Header())
	}
}
func TestAudioWebSocketWireContracts(t *testing.T) {
	c, cfg, _ := fixture(t)
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
