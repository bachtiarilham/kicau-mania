package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	prepareConfigEnv(t)
	t.Setenv("API_TOKEN", strings.Repeat("a", 32))
	t.Setenv("DATA_DIR", t.TempDir())
	t.Setenv("CHANNELS_JSON", `[{"id":"lapak01","source":"rtsp://user:secret@192.0.2.1/feed"}]`)
	c, e := Load(filepath.Join(t.TempDir(), "missing.env"))
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Channels) != 1 {
		t.Fatal("channel configuration not loaded")
	}
	t.Setenv("CHANNELS_JSON", `[{"id":"x","source":"rtsp://192.0.2.1/feed"},{"id":"x","source":"rtsp://192.0.2.1/feed2"}]`)
	if _, e = Load("missing.env"); e == nil {
		t.Fatal("accepted duplicate channels")
	}
	t.Setenv("CHANNELS_JSON", `[{"id":"x","source":"rtsp://192.0.2.1/feed"}]`)
	t.Setenv("AUDIO_ACTIVE_DBFS", "NaN")
	if _, e = Load("missing.env"); e == nil {
		t.Fatal("accepted NaN threshold")
	}
}

// Isolate Load's process-environment behavior from the developer's configuration.
// godotenv.Load also populates the environment, so every key needs cleanup.
func prepareConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"HTTP_ADDR", "API_TOKEN", "DATA_DIR", "RUNTIME_DIR", "FFMPEG_BIN", "MEDIAMTX_BIN",
		"RTSP_ADDR", "RTSP_BASE_URL", "WEBRTC_ADDR", "WEBRTC_PUBLIC_URL", "WEBRTC_LOCAL_UDP_ADDR",
		"WEBRTC_HOSTS", "ALLOWED_ORIGINS", "MEDIA_READ_CIDRS", "CHANNELS_JSON",
		"RESTREAM_TARGETS_JSON", "AUDIO_ACTIVE_DBFS", "MAX_ROUND_DURATION", "MIN_FREE_BYTES",
		"BACKEND_METRICS_URL", "BACKEND_TOKEN", "S3_ENDPOINT", "S3_REGION",
		"S3_BUCKET", "S3_ACCESS_KEY", "S3_SECRET_KEY",
	}
	for _, key := range keys {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("API_TOKEN", strings.Repeat("a", 32))
	t.Setenv("CHANNELS_JSON", `[{"id":"lapak01","source":"rtsp://192.0.2.1/feed"}]`)
}

func TestLoadDefaultsAndDerivedPaths(t *testing.T) {
	prepareConfigEnv(t)
	c, err := Load(filepath.Join(t.TempDir(), "missing.env"))
	if err != nil {
		t.Fatal(err)
	}
	dataDir, err := filepath.Abs("./data")
	if err != nil {
		t.Fatal(err)
	}
	runtimeDir, err := filepath.Abs("./runtime")
	if err != nil {
		t.Fatal(err)
	}
	if c.DataDir != dataDir || c.RuntimeDir != runtimeDir || c.MediaConfig != filepath.Join(runtimeDir, "mediamtx.yml") {
		t.Fatal("default data paths changed")
	}
	if c.Listen != "127.0.0.1:8090" || c.FFmpeg != "ffmpeg" || c.MediaMTX != "mediamtx" || c.RTSPListen != "127.0.0.1:8554" || c.RTSPBase != "rtsp://127.0.0.1:8554" || c.WebRTCListen != ":8889" || c.WebRTCBase != "http://127.0.0.1:8889" {
		t.Fatal("default application/media addresses changed")
	}
	if c.SampleRate != 16000 || c.Threshold != -35 || c.MaxRound != time.Hour || c.MinFreeBytes != 5368709120 {
		t.Fatal("default audio/recording settings changed")
	}
	if c.WebRTCLocalUDP != ":8189" {
		t.Fatal("default WebRTC UDP address changed")
	}
	if strings.Join(c.Origins, ",") != "http://localhost:3000" || strings.Join(c.ICEHosts, ",") != "127.0.0.1" || strings.Join(c.ReadCIDRs, ",") != "127.0.0.1/32,::1/128" {
		t.Fatal("default origin or network lists changed")
	}
	if c.Destinations == nil || len(c.Destinations) != 0 || c.S3Region != "us-east-1" || c.S3Bucket != "" || c.BackendURL != "" {
		t.Fatal("default optional services changed")
	}
}

func TestLoadEnvironmentPrecedenceAndExplicitEmpty(t *testing.T) {
	prepareConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.env")
	contents := "HTTP_ADDR=127.0.0.1:9000\nFFMPEG_BIN=file-ffmpeg\nBACKEND_METRICS_URL=http://metricsclient.test/metrics\nS3_REGION=file-region\nBACKEND_TOKEN=file-token\nDATA_DIR=./custom-data\nAUDIO_ACTIVE_DBFS=-42\nMAX_ROUND_DURATION=90m\nMIN_FREE_BYTES=104857600\nRESTREAM_TARGETS_JSON='{\"youtube\":\"rtmps://example.test/live/key\"}'\n"
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HTTP_ADDR", "127.0.0.1:9001")
	t.Setenv("FFMPEG_BIN", "")
	t.Setenv("BACKEND_METRICS_URL", "")
	t.Setenv("S3_REGION", "")
	t.Setenv("WEBRTC_LOCAL_UDP_ADDR", "127.0.0.1:8190")
	t.Setenv("RUNTIME_DIR", "./custom-runtime")
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "127.0.0.1:9001" || c.FFmpeg != "" || c.BackendURL != "" || c.S3Region != "" {
		t.Fatal("environment overrides, including explicit empty values, changed")
	}
	if c.WebRTCLocalUDP != "127.0.0.1:8190" {
		t.Fatal("WebRTC UDP environment override ignored")
	}
	dataDir, err := filepath.Abs("./custom-data")
	if err != nil {
		t.Fatal(err)
	}
	runtimeDir, err := filepath.Abs("./custom-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if c.DataDir != dataDir || c.RuntimeDir != runtimeDir || c.MediaConfig != filepath.Join(runtimeDir, "mediamtx.yml") || c.BackendToken != "file-token" || c.Threshold != -42 || c.MaxRound != 90*time.Minute || c.MinFreeBytes != 104857600 || c.Destinations["youtube"] != "rtmps://example.test/live/key" {
		t.Fatal("dotenv parsing or derived paths changed")
	}
	t.Setenv("API_TOKEN", "")
	if _, err := Load(path); err == nil || err.Error() != "API_TOKEN requires at least 32 characters" {
		t.Fatal("empty required token was replaced by a fallback")
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		key   string
		value string
	}{
		{"float syntax", "AUDIO_ACTIVE_DBFS", "loud"},
		{"empty runtime directory", "RUNTIME_DIR", ""},
		{"float empty", "AUDIO_ACTIVE_DBFS", ""},
		{"float infinity", "AUDIO_ACTIVE_DBFS", "+Inf"},
		{"float range", "AUDIO_ACTIVE_DBFS", "1"},
		{"duration syntax", "MAX_ROUND_DURATION", "soon"},
		{"duration too short", "MAX_ROUND_DURATION", "59s"},
		{"duration too long", "MAX_ROUND_DURATION", "25h"},
		{"uint syntax", "MIN_FREE_BYTES", "many"},
		{"uint negative", "MIN_FREE_BYTES", "-1"},
		{"uint overflow", "MIN_FREE_BYTES", "18446744073709551616"},
		{"disk minimum", "MIN_FREE_BYTES", "104857599"},
		{"short token", "API_TOKEN", "short"},
		{"placeholder token", "API_TOKEN", "REPLACE_WITH_A_RANDOM_SECRET_AT_LEAST_32_CHARACTERS"},
		{"channels JSON", "CHANNELS_JSON", "{"},
		{"no channels", "CHANNELS_JSON", "[]"},
		{"channel URL", "CHANNELS_JSON", `[{"id":"a","source":"http://camera.test/feed"}]`},
		{"RTSP scheme", "RTSP_BASE_URL", "http://localhost:8554"},
		{"WebRTC scheme", "WEBRTC_PUBLIC_URL", "rtsp://localhost:8889"},
		{"CIDR", "MEDIA_READ_CIDRS", "not-a-network"},
		{"UDP missing port", "WEBRTC_LOCAL_UDP_ADDR", "127.0.0.1"},
		{"UDP invalid port", "WEBRTC_LOCAL_UDP_ADDR", ":invalid"},
		{"UDP port range", "WEBRTC_LOCAL_UDP_ADDR", ":65536"},
		{"UDP port zero", "WEBRTC_LOCAL_UDP_ADDR", ":0"},
		{"origin", "ALLOWED_ORIGINS", "*"},
		{"targets JSON", "RESTREAM_TARGETS_JSON", "{"},
		{"target URL", "RESTREAM_TARGETS_JSON", `{"youtube":"http://example.test/live"}`},
		{"backend URL", "BACKEND_METRICS_URL", "not-a-url"},
		{"S3 URL", "S3_ENDPOINT", "not-a-url"},
		{"S3 missing secret", "S3_ACCESS_KEY", "test-key"},
		{"S3 missing key", "S3_SECRET_KEY", "test-secret"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prepareConfigEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(filepath.Join(t.TempDir(), "missing.env")); err == nil {
				t.Fatalf("invalid %s accepted", tc.key)
			}
		})
	}
}

func TestLoadDoesNotSearchParentDirectories(t *testing.T) {
	prepareConfigEnv(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("AUDIO_ACTIVE_DBFS=invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(child)
	c, err := Load(".env")
	if err != nil || c.Threshold != -35 {
		t.Fatal("Load searched a parent .env")
	}
}
