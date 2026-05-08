package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/redis/go-redis/v9"

	runnerconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-runner/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/cluster"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/executor"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/heartbeat"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/locks"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/logstream"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/queue"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/scheduler"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-runner/internal/worker"
	"github.com/bzyfuzy/bzy-personal/pkg/logging"
	"github.com/bzyfuzy/bzy-personal/pkg/telemetry"
)

func main() {
	app := fx.New(
		fx.Provide(
			runnerconfig.Load,
			provideLogger,
			provideTelemetry,
			provideRedis,
			queue.NewRedisQueue,
			locks.NewRedisLocker,
			logstream.NewRedisStream,
			heartbeat.NewService,
			cluster.NewRegistry,
			executor.NewDockerExecutor,
			executor.NewLocalExecutor,
			worker.NewPool,
			scheduler.NewCronScheduler,
		),
		fx.Invoke(
			startWorkerPool,
			startScheduler,
			startHeartbeat,
		),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := app.Stop(ctx); err != nil {
		os.Exit(1)
	}
}

func provideLogger(cfg *runnerconfig.Config) (*zap.Logger, error) {
	return logging.New(cfg.LogLevel, cfg.LogFormat, cfg.ServiceName)
}

func provideTelemetry(cfg *runnerconfig.Config, lc fx.Lifecycle, logger *zap.Logger) (*telemetry.Provider, error) {
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

func provideRedis(cfg *runnerconfig.Config) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: cfg.Redis.PoolSize,
	})
	return rdb, rdb.Ping(context.Background()).Err()
}

func startWorkerPool(lc fx.Lifecycle, pool *worker.Pool, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go pool.Start(context.Background())
			logger.Info("worker pool started")
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return pool.Shutdown(ctx)
		},
	})
}

func startScheduler(lc fx.Lifecycle, s *scheduler.CronScheduler, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error { s.Start(); return nil },
		OnStop:  func(ctx context.Context) error { return s.Stop(ctx) },
	})
}

func startHeartbeat(lc fx.Lifecycle, hb *heartbeat.Service, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error { go hb.Run(ctx); return nil },
		OnStop:  func(ctx context.Context) error { return hb.Stop(); },
	})
}
