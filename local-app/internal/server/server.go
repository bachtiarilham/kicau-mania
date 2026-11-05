package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/app"
	delivery "kicau.local/edge/internal/delivery/http"
	"kicau.local/edge/internal/storage/filesystem"
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
			return fmt.Errorf("required executable unavailable: %s", bin)
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
	mediaServer := container.MediaServer
	if e = mediaServer.Start(ctx); e != nil {
		container.Saving.Close()
		container.Audio.Close()
		container.Streaming.Close()
		return e
	}
	defer mediaServer.Close()
	stop, e := startModules(container.Saving, container.Audio, container.Streaming)
	if e != nil {
		return e
	}
	defer stop()
	defer container.Handlers.Audio.Close()
	server := &http.Server{Handler: delivery.Router(c, container.Handlers), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	slog.Info("edge server listening", "address", c.Listen, "channels", len(c.Channels))
	select {
	case <-signals:
	case <-mediaServer.Done():
		e = fmt.Errorf("MediaMTX exited; inspect generated configuration and port availability")
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

// Saving is first: recovery precedes other workers, and finalization completes
// before the remaining modules and finally the shared mediaServer are closed.
func startModules(modules ...module) (func(), error) {
	for i, m := range modules {
		if e := m.Start(); e != nil {
			// Also close unstarted modules: their constructors own cancellation contexts.
			for j := len(modules) - 1; j >= 0; j-- {
				modules[j].Close()
			}
			return nil, fmt.Errorf("start module %d: %w", i, e)
		}
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, m := range modules {
				m.Close()
			}
		})
	}, nil
}
