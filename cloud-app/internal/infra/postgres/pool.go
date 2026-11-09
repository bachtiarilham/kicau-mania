// Package postgres owns connection pooling; it does not migrate or query business data.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Options struct {
	URL                string
	MaxConns, MinConns int32
	ConnectTimeout     time.Duration
}

func Open(ctx context.Context, opts Options) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	// Dedicated env options take precedence over pool options inside the URL.
	cfg.MaxConns, cfg.MinConns = opts.MaxConns, opts.MinConns
	cfg.ConnConfig.ConnectTimeout = opts.ConnectTimeout
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("cannot initialize PostgreSQL pool")
	}
	return pool, nil
}
