package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"database/sql"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"

	ifconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-interface/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/auth"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/gateway"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/httpserver"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/middleware"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/session"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/streaming"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/ws"
	"github.com/bzyfuzy/bzy-personal/pkg/health"
	"github.com/bzyfuzy/bzy-personal/pkg/logging"
	"github.com/bzyfuzy/bzy-personal/pkg/telemetry"
)

func main() {
	app := fx.New(
		fx.Provide(
			ifconfig.Load,
			provideLogger,
			provideTelemetry,
			provideRedis,
			provideDB,
			provideHealthChecker,
			provideJWT,
			auth.NewAPIKeyService,
			provideSessionManager,
			provideBrainClient,
			provideRunnerClient,
			provideMiddleware,
			streaming.NewHub,
			ws.NewHub,
			httpserver.RegisterRoutes, // provides *gin.Engine
			httpserver.New,            // consumes *gin.Engine, provides *httpserver.Server
		),
		fx.Invoke(
			startHTTPServer,
		),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-quit; app.Stop(context.Background()) }()
	app.Run()
}

func provideLogger(cfg *ifconfig.Config) (*zap.Logger, error) {
	return logging.New(cfg.LogLevel, cfg.LogFormat, cfg.ServiceName)
}

func provideTelemetry(cfg *ifconfig.Config, lc fx.Lifecycle) (*telemetry.Provider, error) {
	p, shutdown, err := telemetry.Init(context.Background(), telemetry.Config{
		ServiceName:  cfg.ServiceName,
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		SampleRate:   cfg.Telemetry.SampleRate,
		Enabled:      cfg.Telemetry.Enabled,
	})
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{OnStop: func(ctx context.Context) error { return shutdown(ctx) }})
	return p, nil
}

func provideRedis(cfg *ifconfig.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})
	return rdb, rdb.Ping(context.Background()).Err()
}

func provideDB(cfg *ifconfig.Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.Database.DSN)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db, db.PingContext(ctx)
}

func provideHealthChecker(cfg *ifconfig.Config) *health.Checker {
	return health.New(cfg.ServiceName)
}

func provideJWT(cfg *ifconfig.Config) *auth.JWTService {
	return auth.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry, cfg.Auth.RefreshExpiry)
}

func provideSessionManager(cfg *ifconfig.Config, rdb *redis.Client) *session.Manager {
	return session.NewManager(rdb, cfg.Auth.JWTExpiry)
}

func provideBrainClient(cfg *ifconfig.Config) (*gateway.BrainClient, error) {
	return gateway.NewBrainClient(cfg.Gateway.BrainAddr)
}

func provideRunnerClient(cfg *ifconfig.Config) (*gateway.RunnerClient, error) {
	return gateway.NewRunnerClient(cfg.Gateway.RunnerAddr)
}

func provideMiddleware(cfg *ifconfig.Config, jwt *auth.JWTService, apiKeys *auth.APIKeyService, logger *zap.Logger) *middleware.Bundle {
	rps := cfg.RateLimit.RequestsPerMin / 60
	if rps < 1 {
		rps = 1
	}
	return middleware.New(jwt, apiKeys, rps, cfg.RateLimit.BurstSize, logger)
}

func startHTTPServer(lc fx.Lifecycle, srv *httpserver.Server, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := srv.ListenAndServe(); err != nil {
					logger.Fatal("http server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	})
}
