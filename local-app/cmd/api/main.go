package main

import (
	"flag"
	"os"
	"time"
	_ "time/tzdata"

	"go.uber.org/zap"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/app"
	"kicau.local/edge/internal/infra/logger"
	"kicau.local/edge/internal/server"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	time.Local = time.FixedZone("WIB", 7*60*60)
	env := flag.String("env", ".env", "configuration file")
	render := flag.Bool("render-config", false, "write MediaMTX configuration and exit")
	flag.Parse()
	cfg, e := config.Load(*env)
	logger.Init()
	defer logger.Sync()
	if e != nil {
		logger.Error("configuration rejected", zap.Error(e))
		return e
	}
	logger.Info("Bootstrapping local-app", zap.String("arena_id", cfg.ArenaID), zap.String("engine_id", cfg.EngineID))
	if *render {
		_, mediaServer, err := app.NewMedia(cfg)
		e = err
		if e == nil {
			e = mediaServer.Render()
		}
	} else {
		e = server.Run(cfg)
	}
	if e != nil {
		logger.Error("server stopped", zap.Error(e))
	}
	return e
}
