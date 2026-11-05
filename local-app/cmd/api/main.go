package main

import (
	"flag"
	"log/slog"
	"os"
	"time"
	_ "time/tzdata"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/app"
	"kicau.local/edge/internal/server"
)

func main() {
	time.Local = time.FixedZone("WIB", 7*60*60)
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	env := flag.String("env", ".env", "configuration file")
	render := flag.Bool("render-config", false, "write MediaMTX configuration and exit")
	flag.Parse()
	cfg, e := config.Load(*env)
	if e != nil {
		slog.Error("configuration rejected", "error", e)
		os.Exit(1)
	}
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
		slog.Error("server stopped", "error", e)
		os.Exit(1)
	}
}
