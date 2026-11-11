package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"kicau.local/edge/internal/core/identifier"
)

type Config struct {
	AccessMode        string
	LocalControlToken string
	// Aplikasi
	Listen                    string
	Token                     string
	DataDir                   string
	RuntimeDir                string
	Origins                   []string
	ArenaID                   string
	EngineID                  string
	CloudCommandsURL          string
	CloudHeartbeatURL         string
	CloudMediaUpdatesURL      string
	CloudPlaybackAuthorizeURL string
	HeartbeatInterval         time.Duration
	HeartbeatTimeout          time.Duration
	CloudToken                string
	FrontendOrigin            string
	LocalPublicURL            string
	LANServerIP               string
	DNSAddress                string
	DNSUpstreams              []string

	// Media / NVR
	FFmpeg         string
	FFprobe        string
	MediaMTX       string
	MediaConfig    string
	RTSPListen     string
	RTSPBase       string
	WebRTCListen   string
	WebRTCLocalUDP string
	WebRTCBase     string
	ICEHosts       []string
	ReadCIDRs      []string
	Channels       []Channel
	Destinations   map[string]string

	// Audio
	SampleRate int
	Threshold  float64

	// Recording
	MaxRound     time.Duration
	MinFreeBytes uint64

	// Backend
	BackendURL   string
	BackendToken string

	// S3
	S3Endpoint  string
	S3Region    string
	S3Bucket    string
	S3AccessKey string
	S3SecretKey string
}

type Channel struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	LiveMode string `json:"live_mode,omitempty"`
	HasAudio *bool  `json:"has_audio,omitempty"`
}

func Load(path string) (Config, error) {
	if err := godotenv.Load(path); err != nil && !os.IsNotExist(err) {
		return Config{}, err
	}

	// Parse nilai bertipe khusus; nilai invalid tetap menghasilkan error.
	threshold, err := getEnvAsFloat64("AUDIO_ACTIVE_DBFS", "-35")
	if err != nil {
		return Config{}, err
	}
	maxRound, err := getEnvAsDuration("MAX_ROUND_DURATION", "1h")
	if err != nil {
		return Config{}, fmt.Errorf("MAX_ROUND_DURATION must be 1m..24h")
	}
	minFreeBytes, err := getEnvAsUint64("MIN_FREE_BYTES", "5368709120")
	if err != nil {
		return Config{}, fmt.Errorf("MIN_FREE_BYTES requires at least 100 MiB")
	}
	var channels []Channel
	if err := json.Unmarshal([]byte(getEnv("CHANNELS_JSON", "[]")), &channels); err != nil {
		return Config{}, fmt.Errorf("invalid CHANNELS_JSON")
	}
	destinations := map[string]string{}
	if err := json.Unmarshal([]byte(getEnv("RESTREAM_TARGETS_JSON", "{}")), &destinations); err != nil {
		return Config{}, fmt.Errorf("invalid RESTREAM_TARGETS_JSON")
	}

	// Siapkan path turunan sebelum membentuk konfigurasi.
	dataDir, err := filepath.Abs(getEnv("DATA_DIR", "./data"))
	if err != nil {
		return Config{}, err
	}
	runtimePath := getEnv("RUNTIME_DIR", "./runtime")
	if strings.TrimSpace(runtimePath) == "" {
		return Config{}, fmt.Errorf("RUNTIME_DIR must not be empty")
	}
	runtimeDir, err := filepath.Abs(runtimePath)
	if err != nil {
		return Config{}, err
	}

	heartbeatInterval, err := getEnvAsDuration("HEARTBEAT_INTERVAL", "5s")
	if err != nil {
		return Config{}, err
	}
	heartbeatTimeout, err := getEnvAsDuration("HEARTBEAT_TIMEOUT", "15s")
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		AccessMode: getEnv("ENGINE_ACCESS_MODE", "cloud"),
		// Aplikasi
		Listen:                    getEnv("HTTP_ADDR", "127.0.0.1:8090"),
		Token:                     getEnv("API_TOKEN", ""),
		DataDir:                   dataDir,
		RuntimeDir:                runtimeDir,
		Origins:                   strings.Split(getEnv("ALLOWED_ORIGINS", "http://localhost:3000"), ","),
		ArenaID:                   getEnv("ARENA_ID", "arena01"),
		EngineID:                  getEnv("ENGINE_ID", "engine01"),
		CloudCommandsURL:          getEnv("CLOUD_COMMANDS_URL", ""),
		CloudHeartbeatURL:         getEnv("CLOUD_HEARTBEAT_URL", ""),
		CloudMediaUpdatesURL:      getEnv("CLOUD_MEDIA_UPDATES_URL", ""),
		CloudPlaybackAuthorizeURL: getEnv("CLOUD_PLAYBACK_AUTHORIZE_URL", ""),
		HeartbeatInterval:         heartbeatInterval,
		HeartbeatTimeout:          heartbeatTimeout,
		CloudToken:                getEnv("CLOUD_ENGINE_TOKEN", ""),
		FrontendOrigin:            strings.TrimRight(getEnv("FRONTEND_ORIGIN", ""), "/"),
		LocalPublicURL:            strings.TrimRight(getEnv("LOCAL_PUBLIC_URL", ""), "/"),
		LANServerIP:               getEnv("LAN_SERVER_IP", ""),
		DNSAddress:                getEnv("DNS_LISTEN_ADDR", ""),
		DNSUpstreams:              strings.Split(getEnv("DNS_UPSTREAMS", "1.1.1.1:53,8.8.8.8:53"), ","),

		// Media / NVR
		FFmpeg:         getEnv("FFMPEG_BIN", "ffmpeg"),
		FFprobe:        getEnv("FFPROBE_BIN", "ffprobe"),
		MediaMTX:       getEnv("MEDIAMTX_BIN", "mediamtx"),
		MediaConfig:    filepath.Join(runtimeDir, "mediamtx.yml"),
		RTSPListen:     getEnv("RTSP_ADDR", "127.0.0.1:8554"),
		RTSPBase:       getEnv("RTSP_BASE_URL", "rtsp://127.0.0.1:8554"),
		WebRTCListen:   getEnv("WEBRTC_ADDR", "127.0.0.1:8889"),
		WebRTCLocalUDP: getEnv("WEBRTC_LOCAL_UDP_ADDR", ":8189"),
		WebRTCBase:     getEnv("WEBRTC_PUBLIC_URL", "http://127.0.0.1:8090"),
		ICEHosts:       strings.Split(getEnv("WEBRTC_HOSTS", "127.0.0.1"), ","),
		ReadCIDRs:      strings.Split(getEnv("MEDIA_READ_CIDRS", "127.0.0.1/32,::1/128"), ","),
		Channels:       channels,
		Destinations:   destinations,

		// Audio
		SampleRate: 16000,
		Threshold:  threshold,

		// Recording
		MaxRound:     maxRound,
		MinFreeBytes: minFreeBytes,

		// Backend
		BackendURL:   getEnv("BACKEND_METRICS_URL", ""),
		BackendToken: getEnv("BACKEND_TOKEN", ""),

		// S3
		S3Endpoint:  getEnv("S3_ENDPOINT", ""),
		S3Region:    getEnv("S3_REGION", "us-east-1"),
		S3Bucket:    getEnv("S3_BUCKET", ""),
		S3AccessKey: getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey: getEnv("S3_SECRET_KEY", ""),
	}
	if cfg.AccessMode == "lan" && cfg.Token == "" {
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			return Config{}, err
		}
		cfg.Token = hex.EncodeToString(secret[:])
	}
	if cfg.LocalPublicURL != "" {
		cfg.WebRTCBase = cfg.LocalPublicURL
		cfg.Origins = append([]string{cfg.LocalPublicURL}, cfg.Origins...)
	}
	if cfg.FrontendOrigin != "" {
		cfg.Origins = append([]string{cfg.FrontendOrigin}, cfg.Origins...)
	}
	if err := validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validate(c Config) error {
	if c.AccessMode != "" && c.AccessMode != "lan" && c.AccessMode != "cloud" {
		return fmt.Errorf("ENGINE_ACCESS_MODE must be lan or cloud")
	}
	if c.HeartbeatInterval < time.Second || c.HeartbeatTimeout <= c.HeartbeatInterval || c.HeartbeatTimeout > 5*time.Minute {
		return fmt.Errorf("heartbeat timeout must exceed interval and be at most 5m")
	}
	for _, address := range []string{c.WebRTCListen, c.RTSPListen} {
		host, _, err := net.SplitHostPort(address)
		if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
			return fmt.Errorf("RTSP_ADDR and WEBRTC_ADDR must bind loopback")
		}
	}

	if !ValidID(c.ArenaID) || !ValidID(c.EngineID) {
		return fmt.Errorf("invalid ARENA_ID or ENGINE_ID")
	}
	for _, raw := range []string{c.LocalPublicURL, c.FrontendOrigin} {
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("public URLs must be HTTP(S) origins without a path")
		}
	}
	for _, endpoint := range []string{c.CloudCommandsURL, c.CloudHeartbeatURL, c.CloudMediaUpdatesURL, c.CloudPlaybackAuthorizeURL} {
		if endpoint == "" {
			continue
		}
		if !validURL(endpoint, "https", "http") || len(c.CloudToken) < 32 {
			return fmt.Errorf("cloud endpoints require a URL and CLOUD_ENGINE_TOKEN of at least 32 characters")
		}
		u, _ := url.Parse(endpoint)
		if u.User != nil {
			return fmt.Errorf("cloud endpoint must not contain URL credentials")
		}
		if u.Scheme != "https" && u.Hostname() != "localhost" && !net.ParseIP(u.Hostname()).IsLoopback() {
			return fmt.Errorf("cloud credentials require HTTPS outside loopback")
		}
	}
	if c.DNSAddress != "" {
		if c.LocalPublicURL == "" || net.ParseIP(c.LANServerIP).To4() == nil {
			return fmt.Errorf("local DNS requires LOCAL_PUBLIC_URL and LAN_SERVER_IP")
		}
		if _, _, err := net.SplitHostPort(c.DNSAddress); err != nil {
			return fmt.Errorf("invalid DNS_LISTEN_ADDR")
		}
		for _, upstream := range c.DNSUpstreams {
			host, port, err := net.SplitHostPort(strings.TrimSpace(upstream))
			n, _ := strconv.Atoi(port)
			if err != nil || net.ParseIP(host) == nil || n < 1 || n > 65535 {
				return fmt.Errorf("DNS_UPSTREAMS requires IP:port values")
			}
		}
	}
	if !(c.Threshold >= -120 && c.Threshold <= 0) {
		return fmt.Errorf("invalid AUDIO_ACTIVE_DBFS")
	}
	if c.MaxRound < time.Minute || c.MaxRound > 24*time.Hour {
		return fmt.Errorf("MAX_ROUND_DURATION must be 1m..24h")
	}
	if c.MinFreeBytes < 104857600 {
		return fmt.Errorf("MIN_FREE_BYTES requires at least 100 MiB")
	}
	if len(c.Token) < 32 || strings.HasPrefix(c.Token, "REPLACE_") {
		return fmt.Errorf("API_TOKEN requires at least 32 characters")
	}
	if (len(c.Channels) < 1 && c.AccessMode != "lan") || len(c.Channels) > 24 {
		return fmt.Errorf("configure 1..24 channels")
	}
	seen := map[string]bool{}
	for _, ch := range c.Channels {
		if ch.LiveMode != "" && ch.LiveMode != "copy" && ch.LiveMode != "transcode" {
			return fmt.Errorf("invalid live mode for channel %s", ch.ID)
		}
		if !ValidID(ch.ID) || seen[ch.ID] {
			return fmt.Errorf("invalid or duplicate channel ID")
		}
		seen[ch.ID] = true
		if !validURL(ch.Source, "rtsp", "rtsps") {
			return fmt.Errorf("channel %s requires a full RTSP URL", ch.ID)
		}
	}
	if !validURL(c.RTSPBase, "rtsp") || !validURL(c.WebRTCBase, "http", "https") {
		return fmt.Errorf("invalid media base URL")
	}
	_, udpPort, err := net.SplitHostPort(c.WebRTCLocalUDP)
	port, portErr := strconv.Atoi(udpPort)
	if err != nil || portErr != nil || port < 1 || port > 65535 {
		return fmt.Errorf("WEBRTC_LOCAL_UDP_ADDR requires host:port with port 1..65535")
	}
	for _, s := range c.ReadCIDRs {
		if _, _, err := net.ParseCIDR(strings.TrimSpace(s)); err != nil {
			return fmt.Errorf("invalid MEDIA_READ_CIDRS")
		}
	}
	for _, s := range c.Origins {
		if !validURL(s, "http", "https") {
			return fmt.Errorf("ALLOWED_ORIGINS requires explicit HTTP origins")
		}
	}
	for name, target := range c.Destinations {
		if !ValidID(name) || !validURL(target, "rtmp", "rtmps", "rtsp", "rtsps") {
			return fmt.Errorf("invalid restream destination")
		}
	}
	if c.BackendURL != "" && !validURL(c.BackendURL, "http", "https") {
		return fmt.Errorf("invalid BACKEND_METRICS_URL")
	}
	if c.S3Endpoint != "" && !validURL(c.S3Endpoint, "http", "https") {
		return fmt.Errorf("invalid S3_ENDPOINT")
	}
	if (c.S3AccessKey == "") != (c.S3SecretKey == "") {
		return fmt.Errorf("both S3 credentials required")
	}
	return nil
}

// Nilai kosong yang ditetapkan secara eksplisit tetap mengungguli fallback.
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvAsFloat64(key, fallback string) (float64, error) {
	value, err := strconv.ParseFloat(getEnv(key, fallback), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func getEnvAsDuration(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(getEnv(key, fallback))
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func getEnvAsUint64(key, fallback string) (uint64, error) {
	value, err := strconv.ParseUint(getEnv(key, fallback), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return value, nil
}

func ValidID(s string) bool { return identifier.Valid(s) }
func validURL(s string, schemes ...string) bool {
	u, e := url.Parse(s)
	if e != nil || u.Host == "" {
		return false
	}
	for _, v := range schemes {
		if u.Scheme == v {
			return true
		}
	}
	return false
}
