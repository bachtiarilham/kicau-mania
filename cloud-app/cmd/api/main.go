package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"kicau.cloud/control/config"
	"kicau.cloud/control/internal/infra/logger"
	"kicau.cloud/control/internal/server"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("env", ".env", "environment configuration file")
	flag.Parse()
	cfg, err := config.Load(*path)
	logger.Init()
	defer logger.Sync()
	if err != nil {
		logger.Error("configuration rejected", zap.Error(err))
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := server.Run(ctx, cfg); err != nil {
		logger.Error("cloud server stopped", zap.Error(err))
		return err
	}
	return nil
}
