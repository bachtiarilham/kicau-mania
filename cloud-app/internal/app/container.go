// Package app constructs dependencies and closes workers before the database pool.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"kicau.cloud/control/config"
	assetRepo "kicau.cloud/control/internal/asset/repository"
	assetUC "kicau.cloud/control/internal/asset/usecase"
	authRepo "kicau.cloud/control/internal/auth/repository"
	authService "kicau.cloud/control/internal/auth/service"
	authUC "kicau.cloud/control/internal/auth/usecase"
	competitionRepo "kicau.cloud/control/internal/competition/repository"
	competitionUC "kicau.cloud/control/internal/competition/usecase"
	competitionWorker "kicau.cloud/control/internal/competition/worker"
	"kicau.cloud/control/internal/delivery"
	"kicau.cloud/control/internal/delivery/rest"
	assetHandler "kicau.cloud/control/internal/delivery/rest/asset"
	authHandler "kicau.cloud/control/internal/delivery/rest/auth"
	competitionHandler "kicau.cloud/control/internal/delivery/rest/competition"
	iamHandler "kicau.cloud/control/internal/delivery/rest/iam"
	judgingHandler "kicau.cloud/control/internal/delivery/rest/judging"
	health "kicau.cloud/control/internal/health/usecase"
	iamRepo "kicau.cloud/control/internal/iam/repository"
	iamUC "kicau.cloud/control/internal/iam/usecase"
	"kicau.cloud/control/internal/infra/postgres"
	judgingRepo "kicau.cloud/control/internal/judging/repository"
	judgingUC "kicau.cloud/control/internal/judging/usecase"
)

type Container struct {
	Database    *pgxpool.Pool
	Auth        *authUC.UseCase
	IAM         *iamUC.UseCase
	Assets      *assetUC.UseCase
	Competition *competitionUC.UseCase
	Judging     *judgingUC.UseCase
	Reports     *competitionUC.Reports
	Background  *competitionWorker.Worker
	Handlers    delivery.Handlers
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	c := &Container{}
	if cfg.DatabaseURL != "" {
		pool, e := postgres.Open(ctx, postgres.Options{URL: cfg.DatabaseURL, MaxConns: cfg.DBMaxConns, MinConns: cfg.DBMinConns, ConnectTimeout: cfg.DBConnectTimeout})
		if e != nil {
			return nil, e
		}
		c.Database = pool
	}
	db := &postgres.Store{Pool: c.Database}
	authority := iamUC.New(iamRepo.New(db))
	auth := authUC.New(authRepo.New(db), db, authService.Passwords{}, authService.Mailer{Options: authService.MailOptions{Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom, FrontendURL: cfg.FrontendURL, Mode: cfg.SMTPMode, AllowInsecure: cfg.SMTPAllowInsecure}}, authService.Google{ClientID: cfg.GoogleClientID}, authService.NewLimiter())
	assets := assetUC.New(assetRepo.New(db), authority, db)
	cr := competitionRepo.New(db)
	background := competitionWorker.New()
	competition := competitionUC.New(cr, authority, db, background)
	judging := judgingUC.New(judgingRepo.New(db), authority, db)
	reports := competitionUC.NewReports(cr, authority, db)
	c.Auth, c.IAM, c.Assets, c.Competition, c.Judging, c.Reports, c.Background = auth, authority, assets, competition, judging, reports, background
	c.Handlers = delivery.Handlers{Health: rest.NewHealthHandler(health.New(db, cfg.DBHealthTimeout)), Auth: authHandler.New(auth, authHandler.Options{CookieName: cfg.CookieName, Secure: cfg.CookieSecure, Origins: cfg.AllowedOrigins}), IAM: iamHandler.New(authority), Asset: assetHandler.New(assets), Competition: competitionHandler.New(competition, reports), Judging: judgingHandler.New(judging), Assets: assets, Catalog: competition, Criteria: judging, Verifier: auth}
	if c.Database != nil {
		background.Start(ctx, cfg.MVRefreshInterval, cfg.MVRefreshTimeout, reports.Refresh)
	}
	return c, nil
}
func (c *Container) Close() {
	if c.Background != nil {
		c.Background.Close()
	}
	if c.Database != nil {
		c.Database.Close()
	}
}
