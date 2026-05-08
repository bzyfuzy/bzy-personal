package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/fx"
	"go.uber.org/zap"

	ifconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-interface/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/auth"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/gateway"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/httpserver"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/middleware"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/session"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/streaming"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-interface/internal/ws"
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
			auth.NewJWTService,
			auth.NewAPIKeyService,
			session.NewManager,
			gateway.NewBrainClient,
			gateway.NewRunnerClient,
			middleware.New,
			streaming.NewHub,
			ws.NewHub,
			httpserver.New,
		),
		fx.Invoke(
			httpserver.RegisterRoutes,
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
