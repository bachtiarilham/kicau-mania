// Package app wires the active modular Clean Architecture domains.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"kicau.cloud/control/config"
	assetDelivery "kicau.cloud/control/internal/asset/delivery"
	assetRepo "kicau.cloud/control/internal/asset/repository"
	assetUC "kicau.cloud/control/internal/asset/usecase"
	authService "kicau.cloud/control/internal/auth/service"
	competitionDelivery "kicau.cloud/control/internal/competition/delivery"
	competitionRepo "kicau.cloud/control/internal/competition/repository"
	competitionUC "kicau.cloud/control/internal/competition/usecase"
	"kicau.cloud/control/internal/delivery"
	"kicau.cloud/control/internal/delivery/rest"
	mediaDelivery "kicau.cloud/control/internal/delivery/rest/media"
	health "kicau.cloud/control/internal/health/usecase"
	identityDelivery "kicau.cloud/control/internal/identity/delivery"
	identityRepo "kicau.cloud/control/internal/identity/repository"
	identityUC "kicau.cloud/control/internal/identity/usecase"
	"kicau.cloud/control/internal/infra/postgres"
	redisInfra "kicau.cloud/control/internal/infra/redis"
	mediaModel "kicau.cloud/control/internal/media/model"
	mediaService "kicau.cloud/control/internal/media/service"
	mediaUC "kicau.cloud/control/internal/media/usecase"
)

type Container struct {
	Database    *pgxpool.Pool
	Redis       *redisInfra.Store
	Auth        *identityUC.AuthUsecase
	User        *identityUC.UserUsecase
	Asset       *assetUC.AssetUsecase
	Competition *competitionUC.CompetitionUsecase
	Handlers    delivery.Handlers
}

type readiness struct {
	db    *postgres.Store
	redis *redisInfra.Store
}

func (r readiness) Ping(c context.Context) error {
	if err := r.db.Ping(c); err != nil {
		return err
	}
	return r.redis.Ping(c)
}

func New(ctx context.Context, cfg config.Config) (*Container, error) {
	c := &Container{}
	if cfg.DatabaseURL != "" {
		p, err := postgres.Open(ctx, postgres.Options{
			URL:            cfg.DatabaseURL,
			MaxConns:       cfg.DBMaxConns,
			MinConns:       cfg.DBMinConns,
			ConnectTimeout: cfg.DBConnectTimeout,
		})
		if err != nil {
			return nil, err
		}
		c.Database = p
	}
	db := &postgres.Store{Pool: c.Database}

	r, err := redisInfra.New(cfg.RedisURL)
	if err != nil {
		c.Close()
		return nil, err
	}
	c.Redis = r

	p := authService.Passwords{}

	// Identity Domain
	userRepo := identityRepo.NewUserRepository(db)
	sessionRepo := identityRepo.NewSessionRepository(db)
	c.Auth = identityUC.NewAuthUsecase(db, userRepo, sessionRepo, r, p, authService.NewLimiter())
	c.User = identityUC.NewUserUsecase(db, userRepo, p)
	identityHandler := &identityDelivery.Handler{
		Auth:       c.Auth,
		Users:      c.User,
		CookieName: cfg.CookieName,
		Secure:     cfg.CookieSecure,
		Origins:    cfg.AllowedOrigins,
	}

	// Asset Domain
	arenaRepo := assetRepo.NewArenaRepository(db)
	gantangRepo := assetRepo.NewGantanganRepository(db)
	speciesRepo := assetRepo.NewBirdSpeciesRepository(db)
	birdRepo := assetRepo.NewBirdRepository(db)
	catRepo := assetRepo.NewCategoryRepository(db)
	critRepo := assetRepo.NewCriteriaRepository(db)
	c.Asset = assetUC.NewAssetUsecase(db, arenaRepo, gantangRepo, speciesRepo, birdRepo, catRepo, critRepo)
	assetHandler := &assetDelivery.Handler{Asset: c.Asset}

	// Competition Domain
	eventRepo := competitionRepo.NewEventRepository(db)
	roundRepo := competitionRepo.NewRoundRepository(db)
	regRepo := competitionRepo.NewRegistrationRepository(db)
	judgingRepo := competitionRepo.NewJudgingRepository(db)
	c.Competition = competitionUC.NewCompetitionUsecase(db, eventRepo, roundRepo, regRepo, judgingRepo)
	compHandler := &competitionDelivery.Handler{Comp: c.Competition}

	// Media Domain
	var objects mediaModel.Objects
	if cfg.MediaS3Bucket != "" {
		var err error
		objects, err = mediaService.NewObjects(ctx, cfg.MediaS3Endpoint, cfg.MediaS3Region, cfg.MediaS3Bucket, cfg.MediaS3Key, cfg.MediaS3Secret)
		if err != nil {
			c.Close()
			return nil, err
		}
	}
	waSender := mediaService.NewGenericWhatsAppGateway(cfg.WhatsAppEndpoint, cfg.WhatsAppAPIKey)
	mh := &mediaDelivery.Handler{
		UC: &mediaUC.UseCase{
			DB:       db,
			ACL:      c.User,
			Objects:  objects,
			WhatsApp: waSender,
		},
	}

	c.Handlers = delivery.Handlers{
		Media:       mh,
		Health:      rest.NewHealthHandler(health.New(readiness{db, r}, cfg.DBHealthTimeout)),
		Identity:    identityHandler,
		Asset:       assetHandler,
		Competition: compHandler,
		Verifier:    c.Auth,
	}
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
