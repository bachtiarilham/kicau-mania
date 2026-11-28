package app

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	cameraModel "kicau.local/edge/internal/camera/model"
	cameraRepo "kicau.local/edge/internal/camera/repository"
	"kicau.local/edge/internal/infra/ffmpeg"
	"kicau.local/edge/internal/infra/storage/filesystem"
	recordingModel "kicau.local/edge/internal/recording/model"
)

func (m *Coordinator) Arena(ctx context.Context) (config.ArenaSettings, error) {
	settings, err := config.ReadArena(m.base)
	if err != nil {
		return settings, arenaConfigError(err)
	}
	if settings.ServerIP == "" {
		if interfaces := m.Networks(); len(interfaces) > 0 {
			settings.ServerIP = interfaces[0].IP
		}
	}
	settings.Gateway = defaultGateway(ctx, settings.ServerIP)
	return settings, nil
}

func (m *Coordinator) SaveArena(settings config.ArenaSettings) error {
	for _, network := range m.Networks() {
		if network.IP == settings.ServerIP {
			m.ops.Lock()
			defer m.ops.Unlock()
			m.arenaMu.Lock()
			defer m.arenaMu.Unlock()
			if settings.RecordingsDir != nil {
				target := strings.TrimSpace(*settings.RecordingsDir)
				if target != "" {
					target = filepath.Clean(target)
				}
				if target != m.base.RecordingsDir {
					m.mu.RLock()
					active := m.current
					m.mu.RUnlock()
					if active != nil {
						session, err := active.container.Sessions.Current(context.Background())
						if err != nil {
							return err
						}
						if session != nil && !recordingModel.Terminal(session.State) {
							return apperror.Public(apperror.ErrConflict, "Tunggu rekaman dan finalisasi selesai.")
						}
					}
					if target == "" {
						target = filepath.Join(m.base.DataDir, "recordings")
						if err := os.MkdirAll(target, 0700); err != nil {
							return apperror.Public(apperror.ErrInvalid, "Folder rekaman bawaan tidak dapat dibuat.")
						}
					}
					if !filepath.IsAbs(target) || strings.HasPrefix(filepath.VolumeName(target), `\\`) {
						return apperror.Public(apperror.ErrInvalid, "Pilih folder pada drive lokal PC operator.")
					}
					probe, err := os.CreateTemp(target, ".recording-test-*")
					if err != nil {
						return apperror.Public(apperror.ErrInvalid, "Folder tidak tersedia atau Local App tidak dapat menulis ke folder itu.")
					}
					probe.Close()
					os.Remove(probe.Name())
					free, err := (filesystem.Disk{}).Free(target)
					if err != nil || free < m.base.MinFreeBytes {
						return apperror.Public(apperror.ErrInvalid, "Ruang kosong di drive rekaman tidak mencukupi.")
					}
				}
			}
			return arenaConfigError(config.SaveArena(m.base, settings))
		}
	}
	return &apperror.Message{Kind: apperror.ErrInvalid, Text: "Pilih IP yang terpasang pada PC operator."}
}

func (m *Coordinator) TestArenaCamera(ctx context.Context, camera config.ArenaChannel) (any, error) {
	source, err := config.ArenaSource(m.base, camera)
	if err != nil {
		return nil, arenaConfigError(err)
	}
	m.mu.RLock()
	var catalog *cameraRepo.Catalog
	if m.current != nil {
		catalog = m.current.container.ChannelCatalog
	}
	m.mu.RUnlock()
	target, via := arenaProbeTarget(catalog, camera.ID, source)
	probe, err := (ffmpeg.Inspector{ProbeBinary: m.base.FFprobe}).Probe(ctx, target)
	if err != nil {
		label := "Sumber RTSP"
		if via == "mediamtx" {
			label = "Livestream MediaMTX"
		}
		var failure *cameraModel.Failure
		if errors.As(err, &failure) {
			return nil, &apperror.Message{Kind: apperror.ErrUnavailable, Code: failure.Code, Text: label + " · " + failure.Message}
		}
		return nil, &apperror.Message{Kind: apperror.ErrUnavailable, Text: label + " · Tes video tidak selesai."}
	}
	return struct {
		cameraModel.Probe
		Via string `json:"via"`
	}{Probe: probe, Via: via}, nil
}

func arenaProbeTarget(catalog *cameraRepo.Catalog, id, source string) (string, string) {
	if catalog != nil {
		running, err := catalog.Source(id)
		if err == nil {
			active, parseErr := url.Parse(running)
			if parseErr == nil && active.String() == source {
				if relay, err := catalog.HubRTSP(id); err == nil {
					return relay, "mediamtx"
				}
			}
		}
	}
	return source, "direct"
}

func arenaConfigError(err error) error {
	switch {
	case errors.Is(err, config.ErrArenaInvalid):
		return apperror.Public(apperror.ErrInvalid, "Pengaturan arena tidak valid. Periksa IP, duplikat alamat juri, dan URL RTSP.")
	case errors.Is(err, config.ErrArenaConflict):
		return apperror.Public(apperror.ErrConflict, "Konfigurasi sudah berubah. Muat ulang sebelum menyimpan.")
	case errors.Is(err, config.ErrArenaUnavailable):
		return apperror.Public(apperror.ErrUnavailable, "File .env Local App tidak bisa dibaca.")
	default:
		return err
	}
}

func (m *Coordinator) TestArenaNVR(ctx context.Context, ip string) error {
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(ip, "554"))
	if err != nil {
		return &apperror.Message{Kind: apperror.ErrUnavailable, Text: "Port RTSP 554 pada NVR tidak dapat dijangkau."}
	}
	return conn.Close()
}

func defaultGateway(ctx context.Context, interfaceIP string) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "route.exe", "PRINT", "-4").Output()
	if err != nil {
		return ""
	}
	gateway := ""
	var bestMetric int
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 5 || fields[0] != "0.0.0.0" || fields[1] != "0.0.0.0" || fields[3] != interfaceIP {
			continue
		}
		metric, err := strconv.Atoi(fields[4])
		if err == nil && net.ParseIP(fields[2]).To4() != nil && (gateway == "" || metric < bestMetric) {
			gateway, bestMetric = fields[2], metric
		}
	}
	return gateway
}
