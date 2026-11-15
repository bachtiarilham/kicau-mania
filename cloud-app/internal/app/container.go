// Package app wires the active schema-010 application; obsolete modules are not routed.
package app

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"kicau.cloud/control/config"
	authService "kicau.cloud/control/internal/auth/service"
	"kicau.cloud/control/internal/delivery"
	"kicau.cloud/control/internal/delivery/rest"
	handler "kicau.cloud/control/internal/delivery/rest/workflow"
	health "kicau.cloud/control/internal/health/usecase"
	"kicau.cloud/control/internal/infra/postgres"
	redisService "kicau.cloud/control/internal/login/service"
	login "kicau.cloud/control/internal/login/usecase"
	repo "kicau.cloud/control/internal/workflow/repository"
	workflow "kicau.cloud/control/internal/workflow/usecase"
)

type Container struct {
	Database *pgxpool.Pool
	Redis    *redisService.Store
	Auth     *login.UseCase
	Workflow *workflow.UseCase
	Handlers delivery.Handlers
}
type readiness struct {
	db    *postgres.Store
	redis *redisService.Store
}

func (r readiness) Ping(c context.Context) error {
	if e := r.db.Ping(c); e != nil {
		return e
	}
	return r.redis.Ping(c)
}
func New(ctx context.Context, cfg config.Config) (*Container, error) {
	c := &Container{}
	if cfg.DatabaseURL != "" {
		p, e := postgres.Open(ctx, postgres.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns, MinConns: cfg.DBMinConns, ConnectTimeout: cfg.DBConnectTimeout})
		if e != nil {
			return nil, e
		}
		c.Database = p
	}
	db := &postgres.Store{Pool: c.Database}
	r, e := redisService.New(cfg.RedisURL)
	if e != nil {
		c.Close()
		return nil, e
	}
	c.Redis = r
	d := repo.New(db)
	p := authService.Passwords{}
	c.Auth = login.New(d, r, p, authService.NewLimiter())
	c.Workflow = workflow.New(d, p)
	h := &handler.Handler{UC: c.Workflow, Auth: c.Auth, CookieName: cfg.CookieName, Secure: cfg.CookieSecure, Origins: cfg.AllowedOrigins}
	c.Handlers = delivery.Handlers{Health: rest.NewHealthHandler(health.New(readiness{db, r}, cfg.DBHealthTimeout)), Workflow: h, Verifier: c.Auth}
	return c, nil
}
func (c *Container) Close() {
	if c.Redis != nil {
		c.Redis.Close()
	}
	if c.Database != nil {
		c.Database.Close()
	}
}
