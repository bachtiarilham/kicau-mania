package app

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/apperror"
	cameraModel "kicau.local/edge/internal/camera/model"
	"kicau.local/edge/internal/infra/ffmpeg"
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
			m.arenaMu.Lock()
			defer m.arenaMu.Unlock()
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
	probe, err := (ffmpeg.Inspector{ProbeBinary: m.base.FFprobe}).Probe(ctx, source)
	if err != nil {
		var failure *cameraModel.Failure
		if errors.As(err, &failure) {
			return nil, &apperror.Message{Kind: apperror.ErrUnavailable, Code: failure.Code, Text: failure.Message}
		}
		return nil, &apperror.Message{Kind: apperror.ErrUnavailable, Text: "Tes video tidak selesai."}
	}
	return probe, nil
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
