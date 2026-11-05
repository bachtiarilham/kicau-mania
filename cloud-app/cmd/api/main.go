package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"kicau.cloud/control/internal/exchange/delivery"
	"kicau.cloud/control/internal/exchange/model"
	"kicau.cloud/control/internal/exchange/repository"
	"kicau.cloud/control/internal/exchange/usecase"
)

func main() {
	if err := run(); err != nil {
		slog.Error("cloud receiver stopped")
		os.Exit(1)
	}
}
func run() error {
	env := flag.String("env", ".env", "cloud receiver configuration")
	flag.Parse()
	if err := godotenv.Load(*env); err != nil && !os.IsNotExist(err) {
		return err
	}
	var engines []model.Engine
	if err := json.Unmarshal([]byte(os.Getenv("CLOUD_ENGINES_JSON")), &engines); err != nil {
		return err
	}
	db := os.Getenv("CLOUD_DATABASE")
	if db == "" {
		db = "./data/exchange.sqlite"
	}
	store, err := repository.Open(db)
	if err != nil {
		return err
	}
	defer store.Close()
	uc, err := usecase.New(store, engines, os.Getenv("CLOUD_ADMIN_TOKEN"))
	if err != nil {
		return err
	}
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8091"
	}
	server := &http.Server{Addr: address, Handler: delivery.Router(uc), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errorsCh := make(chan error, 1)
	go func() { errorsCh <- server.ListenAndServe() }()
	slog.Info("cloud exchange receiver listening", "address", address)
	select {
	case <-ctx.Done():
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	return server.Shutdown(shutdown)
}
