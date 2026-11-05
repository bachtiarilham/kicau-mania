package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/app"
	delivery "kicau.local/edge/internal/delivery/http"
	localDNS "kicau.local/edge/internal/infra/network/dns"
	"kicau.local/edge/internal/infra/storage/filesystem"
)

func Run(c config.Config) error {
	if e := os.MkdirAll(c.DataDir, 0700); e != nil {
		return e
	}
	lock, e := filesystem.Lock(filepath.Join(c.DataDir, "edge.lock"))
	if e != nil {
		return fmt.Errorf("cannot lock DATA_DIR; another server may be running")
	}
	defer lock.Close()
	if e := os.MkdirAll(c.RuntimeDir, 0700); e != nil {
		return e
	}
	runtimeLock, e := filesystem.Lock(filepath.Join(c.RuntimeDir, "mediamtx.lock"))
	if e != nil {
		return fmt.Errorf("cannot lock RUNTIME_DIR; use a separate directory for each server")
	}
	defer runtimeLock.Close()
	for _, bin := range []string{c.FFmpeg, c.MediaMTX} {
		if _, e := exec.LookPath(bin); e != nil {
			slog.Warn("media executable unavailable; arena service will continue")
		}
	}
	// Bind before recovery so a second server cannot mutate the same queue accidentally.
	listener, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return e
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	container, e := app.NewContainer(ctx, c)
	if e != nil {
		return e
	}
	defer container.ArenaStore.Close()
	if c.DNSAddress != "" {
		public, _ := url.Parse(c.LocalPublicURL)
		resolver, err := localDNS.Start(localDNS.Options{Address: c.DNSAddress, Host: public.Hostname(), IP: c.LANServerIP, Upstreams: c.DNSUpstreams, AllowedCIDRs: c.ReadCIDRs})
		if err != nil {
			slog.Error("local DNS unavailable; HTTP and judging remain running")
		} else {
			defer resolver.Close()
		}
	}
	if e := container.Background.Start(); e != nil {
		return e
	}
	defer container.Background.Close()
	if e := container.Background.Loop("cloud-sync", 5*time.Second, container.Synchronization.Flush); e != nil {
		return e
	}
	if e := container.Background.Loop("cloud-commands", 5*time.Second, container.Commands.Poll); e != nil {
		return e
	}
	if c.BackupDir != "" {
		if e := container.Background.Loop("backup", 5*time.Minute, func(ctx context.Context) error { _, err := container.Arena.Backup(ctx); return err }); e != nil {
			return e
		}
	} else {
		slog.Warn("BACKUP_DIR not configured; provision separate backup storage before the event")
	}
	mediaServer := container.MediaServer
	mediaCtx, stopMedia := context.WithCancel(ctx)
	mediaDone := superviseMedia(mediaCtx, mediaServer, 5*time.Second)
	defer func() { stopMedia(); <-mediaDone }()
	stop := startIndependentModules(container.Saving, container.Audio, container.Streaming)
	defer stop()
	defer container.Handlers.Audio.Close()
	// Stop new cloud commands before finalizing/stopping the media modules.
	defer container.Background.Close()
	server := &http.Server{Handler: delivery.Router(c, container.Handlers), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	slog.Info("edge server listening", "address", c.Listen, "channels", len(c.Channels))
	select {
	case <-signals:
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			e = err
		}
	}
	shutdown, stopShutdown := context.WithTimeout(context.Background(), 40*time.Second)
	defer stopShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		if e == nil {
			e = err
		}
	}
	return e
}

type module interface {
	Start() error
	Close()
}
