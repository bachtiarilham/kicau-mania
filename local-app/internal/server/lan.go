package server

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
	"kicau.local/edge/config"
	"kicau.local/edge/internal/app"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/infra/storage/filesystem"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func runLAN(c config.Config) error {
	for _, dir := range []string{c.DataDir, c.RuntimeDir} {
		if e := os.MkdirAll(dir, 0700); e != nil {
			return e
		}
	}
	lock, e := filesystem.Lock(filepath.Join(c.DataDir, "edge.lock"))
	if e != nil {
		return fmt.Errorf("another local-app instance is running")
	}
	defer lock.Close()
	mediaLock, e := filesystem.Lock(filepath.Join(c.RuntimeDir, "mediamtx.lock"))
	if e != nil {
		return fmt.Errorf("media runtime directory is in use")
	}
	defer mediaLock.Close()
	listener, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return fmt.Errorf("cannot bind HTTP_ADDR; check that the configured port is free")
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager, e := app.NewCoordinator(ctx, c)
	if e != nil {
		return e
	}
	restoreCtx, stopRestore := context.WithCancel(ctx)
	restoreDone := make(chan struct{})
	defer func() { stopRestore(); manager.Close(); <-restoreDone }()
	server := &http.Server{Handler: manager, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	go func() {
		defer close(restoreDone)
		for restoreCtx.Err() == nil {
			manager.Restore()
			timer := time.NewTimer(5 * time.Second)
			select {
			case <-restoreCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	_, port, _ := net.SplitHostPort(c.Listen)
	address := "http://127.0.0.1:" + port
	logger.Info("LAN API available; frontend is provided separately", zap.String("url", address))
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case <-signals:
	case e = <-done:
		if errors.Is(e, http.ErrServerClosed) {
			e = nil
		}
	}
	shutdown, stop := context.WithTimeout(context.Background(), 40*time.Second)
	defer stop()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		if e == nil {
			e = err
		}
	}
	return e
}
