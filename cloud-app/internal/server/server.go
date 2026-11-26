// Package server runs the HTTP process and coordinates graceful shutdown.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"kicau.cloud/control/config"
	"kicau.cloud/control/internal/app"
	delivery "kicau.cloud/control/internal/delivery"
	"kicau.cloud/control/internal/infra/logger"
	"kicau.cloud/control/internal/infra/telemetry"
)

func Run(ctx context.Context, cfg config.Config) error {
	listener, err := net.Listen("tcp", cfg.HTTPAddress)
	if err != nil {
		return errors.New("cannot bind HTTP_ADDR")
	}
	defer listener.Close()
	container, err := app.New(ctx, cfg)
	if err != nil {
		return err
	}
	defer container.Close()
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "cloud-app"
	}
	shutdownTracer, tracerErr := telemetry.InitAlloyTracer(ctx, serviceName)
	if tracerErr != nil {
		logger.Warn("OpenTelemetry tracer init failed; continuing without tracing", zap.Error(tracerErr))
	} else if shutdownTracer != nil {
		defer func() {
			tCtx, tCancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer tCancel()
			_ = shutdownTracer(tCtx)
		}()
	}
	if container.Database == nil {
		logger.Warn("PostgreSQL is not configured; readiness will return 503")
	}
	server := &http.Server{
		Handler:           delivery.Router(delivery.Options{AllowedOrigins: cfg.AllowedOrigins, TrustedProxies: cfg.TrustedProxies, RequestTimeout: cfg.RequestTimeout}, container.Handlers),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: cfg.RequestTimeout,
		WriteTimeout: cfg.RequestTimeout + time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768,
	}
	stopped := make(chan error, 1)
	go func() { stopped <- server.Serve(listener) }()
	logger.Info("cloud HTTP server listening", zap.String("address", cfg.HTTPAddress))
	select {
	case err := <-stopped:
		if !errors.Is(err, http.ErrServerClosed) {
			_ = server.Close()
			return errors.New("HTTP server failed")
		}
		return nil
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil {
		_ = server.Close()
		return errors.New("HTTP shutdown timed out")
	}
	logger.Info("cloud HTTP server stopped")
	return nil
}
