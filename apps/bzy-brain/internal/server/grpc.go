// Package server exposes bzy-brain's gRPC API.
package server

import (
	"fmt"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	brainconfig "github.com/bzyfuzy/bzy-personal/apps/bzy-brain/config"
)

// GRPCServer wraps the gRPC server instance.
type GRPCServer struct {
	srv    *grpc.Server
	addr   string
	logger *zap.Logger
}

func NewGRPC(cfg *brainconfig.Config, logger *zap.Logger) *GRPCServer {
	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			loggingInterceptor(logger),
			recoveryInterceptor(logger),
		),
	)

	// Register health check service
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("bzy.brain.v1.BrainService", grpc_health_v1.HealthCheckResponse_SERVING)

	// Enable server reflection for grpcurl / tooling
	reflection.Register(srv)

	return &GRPCServer{srv: srv, addr: cfg.GRPC.Addr, logger: logger}
}

// RegisterGRPCHandlers wires service implementations to the gRPC server.
func RegisterGRPCHandlers(grpcSrv *GRPCServer) {
	// Registration happens here when proto-generated code is available:
	// brainpb.RegisterBrainServiceServer(grpcSrv.srv, handlers.NewBrainHandler(...))
}

func (s *GRPCServer) Serve() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("grpc listen %s: %w", s.addr, err)
	}
	s.logger.Info("bzy-brain gRPC listening", zap.String("addr", s.addr))
	return s.srv.Serve(lis)
}

func (s *GRPCServer) GracefulStop() { s.srv.GracefulStop() }
func (s *GRPCServer) Stop()         { s.srv.Stop() }

// ─── Interceptors ─────────────────────────────────────────────────────────────

func loggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx interface{}, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		logger.Debug("grpc call", zap.String("method", info.FullMethod))
		return handler(ctx, req)
	}
}

func recoveryInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx interface{}, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("grpc panic", zap.Any("panic", r), zap.String("method", info.FullMethod))
				err = fmt.Errorf("internal server error")
			}
		}()
		return handler(ctx, req)
	}
}
