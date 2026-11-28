package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joho/godotenv"
)

const ArenaCameraID = "arena-overview"

var (
	ErrArenaInvalid     = errors.New("invalid arena configuration")
	ErrArenaConflict    = errors.New("arena configuration changed")
	ErrArenaUnavailable = errors.New("arena configuration unavailable")
)

type ArenaSettings struct {
	ServerIP               string         `json:"server_ip"`
	JudgeIPs               []string       `json:"judge_ips"`
	NVRIP                  string         `json:"nvr_ip"`
	Gateway                string         `json:"gateway"`
	Channels               []ArenaChannel `json:"channels"`
	Revision               string         `json:"revision"`
	RecordingsDir          *string        `json:"recordings_dir,omitempty"`
	EffectiveRecordingsDir string         `json:"effective_recordings_dir,omitempty"`
}

type ArenaChannel struct {
	ID             string `json:"id"`
	IP             string `json:"ip"`
	Source         string `json:"source"`
	HasCredentials bool   `json:"has_credentials"`
}

func ReadArena(base Config) (ArenaSettings, error) {
	settings, channels, _, err := readArena(base)
	if err != nil {
		return ArenaSettings{}, err
	}
	settings.Channels = make([]ArenaChannel, 0, len(channels))
	for _, channel := range channels {
		source, _ := url.Parse(channel.Source)
		hasCredentials := source.User != nil
		ip := channel.IP
		if ip == "" {
			ip = source.Hostname()
		}
		settings.Channels = append(settings.Channels, ArenaChannel{ID: channel.ID, IP: ip, Source: source.String(), HasCredentials: hasCredentials})
	}
	return settings, nil
}

func ArenaSource(base Config, camera ArenaChannel) (string, error) {
	_, saved, _, err := readArena(base)
	if err != nil {
		return "", err
	}
	source, err := arenaSource(camera, saved)
	if err != nil {
		return "", err
	}
	return source.String(), nil
}

func SaveArena(base Config, settings ArenaSettings) error {
	current, saved, original, err := readArena(base)
	if err != nil {
		return err
	}
	if settings.Revision != current.Revision {
		return ErrArenaConflict
	}
	directory := *current.RecordingsDir
	if settings.RecordingsDir != nil {
		directory = strings.TrimSpace(*settings.RecordingsDir)
		if directory != "" && !localRecordingDirectory(directory) {
			return ErrArenaInvalid
		}
		if directory != "" {
			directory = filepath.Clean(directory)
		}
	}
	base.RecordingsDir = directory
	if !arenaIPv4(settings.ServerIP) || settings.NVRIP != "" && !arenaIPv4(settings.NVRIP) {
		return ErrArenaInvalid
	}
	if len(settings.JudgeIPs) > 4 || len(settings.Channels) > 25 {
		return ErrArenaInvalid
	}
	judges := make([]string, 0, len(settings.JudgeIPs))
	seen := map[string]bool{settings.ServerIP: true}
	for _, judge := range settings.JudgeIPs {
		if !arenaIPv4(judge) || seen[judge] {
			return ErrArenaInvalid
		}
		seen[judge] = true
		judges = append(judges, judge+"/32")
	}
	channels := make([]Channel, 0, len(settings.Channels))
	for _, camera := range settings.Channels {
		if camera.Source == "" {
			continue
		}
		source, err := arenaSource(camera, saved)
		if err != nil {
			return err
		}
		ip := camera.IP
		if ip == "" {
			ip = source.Hostname()
		}
		if !arenaIPv4(ip) && ip != source.Hostname() {
			return ErrArenaInvalid
		}
		next := Channel{ID: camera.ID, IP: ip, Source: source.String()}
		for _, previous := range saved {
			if previous.ID == next.ID {
				next.MAC, next.HasAudio = previous.MAC, previous.HasAudio
				break
			}
		}
		channels = append(channels, next)
	}
	base.LANServerIP, base.NVRIP, base.JudgeReadCIDRs, base.Channels = settings.ServerIP, settings.NVRIP, judges, channels
	_, rtspPort, err := net.SplitHostPort(base.RTSPListen)
	if err != nil {
		return ErrArenaInvalid
	}
	rtspHost := "127.0.0.1"
	if len(judges) > 0 {
		rtspHost = "0.0.0.0"
	}
	base.RTSPListen = net.JoinHostPort(rtspHost, rtspPort)
	if len(judges) > 0 {
		httpHost, httpPort, err := net.SplitHostPort(base.Listen)
		if err != nil {
			return ErrArenaInvalid
		}
		if strings.EqualFold(httpHost, "localhost") || net.ParseIP(httpHost).IsLoopback() {
			base.Listen = net.JoinHostPort("0.0.0.0", httpPort)
		}
	}
	if validate(base) != nil {
		return ErrArenaInvalid
	}
	encoded, err := json.Marshal(channels)
	if err != nil {
		return err
	}
	updated, err := patchEnv(original, map[string]string{
		"LAN_SERVER_IP": base.LANServerIP, "NVR_IP": base.NVRIP,
		"JUDGE_READ_CIDRS": strings.Join(judges, ","), "CHANNELS_JSON": string(encoded),
		"HTTP_ADDR": base.Listen, "RTSP_ADDR": base.RTSPListen,
		"RECORDINGS_DIR": base.RecordingsDir,
	})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(base.EnvPath), ".arena-env-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.WriteString(updated)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), base.EnvPath)
}

func localRecordingDirectory(dir string) bool {
	volume := filepath.VolumeName(dir)
	return filepath.IsAbs(dir) && !strings.HasPrefix(volume, `\\`) &&
		!strings.ContainsAny(dir, "\x00\r\n")
}

func arenaIPv4(raw string) bool {
	addr, err := netip.ParseAddr(raw)
	return err == nil && addr.Is4() && !addr.IsUnspecified() && !addr.IsMulticast()
}

func arenaSource(camera ArenaChannel, saved []Channel) (*url.URL, error) {
	if !ValidID(camera.ID) || !validURL(camera.Source, "rtsp", "rtsps") {
		return nil, ErrArenaInvalid
	}
	source, _ := url.Parse(camera.Source)
	if source.User != nil {
		return source, nil
	}
	for _, previous := range saved {
		if previous.ID == camera.ID {
			old, _ := url.Parse(previous.Source)
			if old != nil && strings.EqualFold(old.Host, source.Host) {
				source.User = old.User
			}
			break
		}
	}
	return source, nil
}

func readArena(base Config) (ArenaSettings, []Channel, string, error) {
	raw, err := os.ReadFile(base.EnvPath)
	if err != nil {
		return ArenaSettings{}, nil, "", ErrArenaUnavailable
	}
	values, err := godotenv.UnmarshalBytes(raw)
	if err != nil {
		return ArenaSettings{}, nil, "", ErrArenaInvalid
	}
	configuredDir := base.RecordingsDir
	if value, found := values["RECORDINGS_DIR"]; found {
		configuredDir = value
	}
	effectiveDir := base.RecordingsDir
	if effectiveDir == "" {
		effectiveDir = filepath.Join(base.DataDir, "recordings")
	}
	settings := ArenaSettings{ServerIP: base.LANServerIP, NVRIP: base.NVRIP, JudgeIPs: []string{}, RecordingsDir: &configuredDir, EffectiveRecordingsDir: effectiveDir}
	if value, found := values["LAN_SERVER_IP"]; found {
		settings.ServerIP = value
	}
	if value, found := values["NVR_IP"]; found {
		settings.NVRIP = value
	}
	judges := base.JudgeReadCIDRs
	if value, found := values["JUDGE_READ_CIDRS"]; found {
		judges = strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' })
	}
	for _, cidr := range judges {
		if address, found := strings.CutSuffix(cidr, "/32"); found {
			settings.JudgeIPs = append(settings.JudgeIPs, address)
		}
	}
	channels := base.Channels
	if value, found := values["CHANNELS_JSON"]; found {
		if err := json.Unmarshal([]byte(value), &channels); err != nil {
			return ArenaSettings{}, nil, "", ErrArenaInvalid
		}
	}
	for _, ch := range channels {
		if !validURL(ch.Source, "rtsp", "rtsps") {
			return ArenaSettings{}, nil, "", ErrArenaInvalid
		}
	}
	hash := sha256.Sum256(raw)
	settings.Revision = hex.EncodeToString(hash[:])
	return settings, channels, string(raw), nil
}

func patchEnv(original string, changes map[string]string) (string, error) {
	old, err := godotenv.Unmarshal(original)
	if err != nil {
		return "", ErrArenaInvalid
	}
	result := original
	for _, key := range slices.Sorted(maps.Keys(changes)) {
		value := changes[key]
		line, err := godotenv.Marshal(map[string]string{key: value})
		if err != nil {
			return "", err
		}
		pattern := regexp.MustCompile(`(?m)^[\t ]*(?:export[\t ]+)?` + key + `[\t ]*=[\t ]*(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\r\n]*)[\t ]*(?:#[^\r\n]*)?`)
		matches := pattern.FindAllStringIndex(result, -1)
		switch len(matches) {
		case 0:
			result = strings.TrimRight(result, "\r\n") + "\n" + line + "\n"
		case 1:
			result = result[:matches[0][0]] + line + result[matches[0][1]:]
		default:
			return "", ErrArenaInvalid
		}
	}
	saved, err := godotenv.Unmarshal(result)
	if err != nil {
		return "", ErrArenaInvalid
	}
	for key, value := range old {
		if next, changed := changes[key]; changed {
			value = next
		}
		if saved[key] != value {
			return "", fmt.Errorf("arena configuration cannot preserve %s: %w", key, ErrArenaInvalid)
		}
	}
	for key, value := range changes {
		if saved[key] != value {
			return "", ErrArenaInvalid
		}
	}
	return result, nil
}
