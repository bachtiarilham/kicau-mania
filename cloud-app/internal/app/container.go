// Package app constructs application dependencies and owns their cleanup.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"kicau.cloud/control/config"
	delivery "kicau.cloud/control/internal/delivery"
	"kicau.cloud/control/internal/delivery/rest"
	health "kicau.cloud/control/internal/health/usecase"
	"kicau.cloud/control/internal/infra/postgres"
)

type Container struct {
	Database *pgxpool.Pool
	Handlers delivery.Handlers
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	c := &Container{}
	var database health.Database
	if cfg.DatabaseURL != "" {
		pool, err := postgres.Open(ctx, postgres.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns, MinConns: cfg.DBMinConns, ConnectTimeout: cfg.DBConnectTimeout})
		if err != nil {
			return nil, err
		}
		c.Database, database = pool, pool
	}
	c.Handlers.Health = rest.NewHealthHandler(health.New(database, cfg.DBHealthTimeout))
	return c, nil
}

func (c *Container) Close() {
	if c.Database != nil {
		c.Database.Close()
	}
}
