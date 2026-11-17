package app

import (
	"context"
	"encoding/json"
	"fmt"
	"kicau.local/edge/config"
	"kicau.local/edge/internal/infra/storage/credentials"
	ffmpegMedia "kicau.local/edge/internal/mediaengine/media/ffmpeg"
	session "kicau.local/edge/internal/mediaengine/session/model"
	setupRepo "kicau.local/edge/internal/setup/repository"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDesktopRealSaving(t *testing.T) {
	source, ffmpeg, probe, relay := os.Getenv("KICAU_TEST_NVR_RTSP_URL"), os.Getenv("KICAU_TEST_FFMPEG"), os.Getenv("KICAU_TEST_FFPROBE"), os.Getenv("KICAU_TEST_MEDIAMTX")
	if source == "" && os.Getenv("KICAU_TEST_NVR_SETTINGS_FILE") != "" {
		stored, err := setupRepo.New(os.Getenv("KICAU_TEST_NVR_SETTINGS_FILE"), credentials.Protector{}).Load(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, ch := range stored.Channels {
			if os.Getenv("KICAU_TEST_NVR_CHANNEL") == "" || ch.ID == os.Getenv("KICAU_TEST_NVR_CHANNEL") {
				source = ch.Source
				break
			}
		}
	}
	if source == "" || ffmpeg == "" || probe == "" || relay == "" {
		t.Skip("Explicit isolated native media binaries/source required")
	}
	address := func() string {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		a := l.Addr().String()
		l.Close()
		return a
	}
	rtsp := address()
	sourceProbe, e := (ffmpegMedia.Inspector{ProbeBinary: probe}).Probe(context.Background(), source)
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	cfg := config.Config{AccessMode: "lan", Listen: "127.0.0.1:8090", LocalControlToken: strings.Repeat("n", 64), Token: strings.Repeat("t", 64), ArenaID: "native-arena", EngineID: "native-engine", DataDir: dir, RuntimeDir: filepath.Join(dir, "runtime"), MediaConfig: filepath.Join(dir, "runtime", "mediamtx.json"), MediaMTX: relay, FFmpeg: ffmpeg, FFprobe: probe, RTSPListen: rtsp, RTSPBase: "rtsp://" + rtsp, ReadCIDRs: []string{"127.0.0.1/32"}, Channels: []config.Channel{{ID: "g01", Source: source}}, SampleRate: 16000, Threshold: -35, MaxRound: time.Minute, MinFreeBytes: 1}
	runtime, e := startRuntime(context.Background(), cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if runtime != nil {
			runtime.close()
		}
	}()
	ready := false
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		conn, e := net.DialTimeout("tcp", rtsp, 100*time.Millisecond)
		if e == nil {
			conn.Close()
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		diagnostic, _ := exec.CommandContext(ctx, relay, cfg.MediaConfig).CombinedOutput()
		t.Fatalf("isolated media relay not listening: %s", diagnostic)
	}
	uc := runtime.container.Sessions
	id := fmt.Sprintf("native-saving-%d", time.Now().UnixNano())
	if _, e = uc.StartSession(context.Background(), session.StartRequest{ID: id, Channels: []string{"g01"}}); e != nil {
		t.Fatal(e)
	}
	running := false
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		s, e := uc.Get(context.Background(), id)
		if e != nil {
			t.Fatal(e)
		}
		if s.State == "RUNNING" {
			running = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !running {
		s, _ := uc.Get(context.Background(), id)
		t.Fatalf("real recording did not reach RUNNING: %+v", s)
	}
	time.Sleep(3 * time.Second)
	if e := runtime.container.MediaServer.Restart(); e != nil {
		t.Fatal(e)
	}
	reconnected := false
	for deadline := time.Now().Add(35 * time.Second); time.Now().Before(deadline); {
		state, _ := uc.Get(context.Background(), id)
		if state.State == "RUNNING" && len(state.ChannelsState) > 0 && state.ChannelsState[0].Attempts >= 2 {
			reconnected = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !reconnected {
		t.Fatal("isolated NVR relay did not reconnect")
	}
	time.Sleep(3 * time.Second)
	if _, e = uc.StopSession(context.Background(), id); e != nil {
		t.Fatal(e)
	}
	stopped := false
	for deadline := time.Now().Add(25 * time.Second); time.Now().Before(deadline); {
		s, e := uc.Get(context.Background(), id)
		if e != nil {
			t.Fatal(e)
		}
		if session.Terminal(s.State) {
			if s.State != "STOPPED" {
				t.Fatalf("finalization failed: %+v", s)
			}
			stopped = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !stopped {
		t.Fatal("finalization timeout")
	}
	records, _, e := runtime.container.Saving.Recordings(context.Background(), "", 25)
	if e != nil || len(records) != 1 || records[0].State != "READY" {
		t.Fatalf("invalid local saving: %+v %v", records, e)
	}
	if filepath.Ext(records[0].File) != ".mkv" {
		t.Fatal("recording must use MKV")
	}
	raw, e := exec.Command(probe, "-v", "error", "-show_streams", "-of", "json", records[0].File).Output()
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Streams []struct {
			Type   string `json:"codec_type"`
			Codec  string `json:"codec_name"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
			Rate   string `json:"r_frame_rate"`
		}
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		t.Fatal(e)
	}
	video, audio := false, false
	for _, s := range result.Streams {
		parts := strings.Split(s.Rate, "/")
		fpsOK := sourceProbe.FPS == 0
		if len(parts) == 2 {
			n, _ := strconv.ParseFloat(parts[0], 64)
			d, _ := strconv.ParseFloat(parts[1], 64)
			if d > 0 {
				fpsOK = fpsOK || math.Abs(n/d-sourceProbe.FPS) < 0.1
			}
		}
		video = video || s.Type == "video" && s.Codec == "hevc" && s.Width == sourceProbe.Width && s.Height == sourceProbe.Height && fpsOK
		audio = audio || s.Type == "audio" && s.Codec == sourceProbe.AudioCodec
	}
	if !video || sourceProbe.HasAudio && !audio {
		t.Fatal("MKV must preserve HEVC resolution and original audio")
	}
	runtime.close()
	runtime = nil
	restarted, err := startRuntime(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.close()
	recovered, _, err := restarted.container.Saving.Recordings(context.Background(), "", 25)
	if err != nil || len(recovered) != 1 || recovered[0].State != "READY" {
		t.Fatal("NVR recording did not survive engine restart", err)
	}
	t.Logf("NVR verified: HEVC %dx%d %.2f fps, original audio %s; relay reconnect and engine restart passed", sourceProbe.Width, sourceProbe.Height, sourceProbe.FPS, sourceProbe.AudioCodec)

}
