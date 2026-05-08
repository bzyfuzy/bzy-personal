package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	brainconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-brain/config"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/agents"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/embedding"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/memory"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/planning"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/providers"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/rag"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/reasoning"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/server"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/tools"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/vectorstore"
	"github.com/bzyfuzy/bzy-personal/apps/bzy-brain/internal/workflows"
	"github.com/bzyfuzy/bzy-personal/pkg/logging"
	"github.com/bzyfuzy/bzy-personal/pkg/telemetry"
)

func main() {
	app := fx.New(
		fx.Provide(
			brainconfig.Load,
			provideLogger,
			provideTelemetry,
			providers.NewRegistry,
			embedding.NewService,
			vectorstore.NewPgVector,
			memory.NewService,
			memory.NewRepository,
			rag.NewPipeline,
			reasoning.NewEngine,
			planning.NewPlanner,
			agents.NewRegistry,
			tools.NewRegistry,
			workflows.NewEngine,
			server.NewGRPC,
		),
		fx.Invoke(
			server.RegisterGRPCHandlers,
			startServer,
		),
	)

	app.Run()
}

func provideLogger(cfg *brainconfig.Config) (*zap.Logger, error) {
	return logging.New(cfg.LogLevel, cfg.LogFormat, cfg.ServiceName)
}

func provideTelemetry(cfg *brainconfig.Config, lc fx.Lifecycle, logger *zap.Logger) (*telemetry.Provider, error) {
	ctx := context.Background()
	p, shutdown, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:  cfg.ServiceName,
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		SampleRate:   cfg.Telemetry.SampleRate,
		Enabled:      cfg.Telemetry.Enabled,
	})
	if err != nil {
		return nil, err
	}
	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("shutting down telemetry")
			return shutdown(ctx)
		},
	})
	return p, nil
}

func startServer(lc fx.Lifecycle, grpc *server.GRPCServer, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := grpc.Serve(); err != nil {
					logger.Fatal("grpc server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("graceful shutdown: draining in-flight requests")
			stop := make(chan struct{})
			go func() {
				grpc.GracefulStop()
				close(stop)
			}()
			select {
			case <-stop:
			case <-time.After(15 * time.Second):
				grpc.Stop()
			}
			return nil
		},
	})

	// Handle OS signals outside fx for clean dev experience
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		logger.Info("signal received — initiating shutdown")
	}()
}
