// Package telemetry configures OpenTelemetry tracing and Prometheus metrics.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config controls telemetry bootstrap.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string  // gRPC endpoint e.g. "localhost:4317"
	SampleRate     float64 // 0.0–1.0
	Enabled        bool
}

// Provider wraps the OTel SDK provider.
type Provider struct {
	tp *sdktrace.TracerProvider
}

// Init bootstraps OpenTelemetry. Returns shutdown func to call on exit.
func Init(ctx context.Context, cfg Config) (*Provider, func(context.Context) error, error) {
	if !cfg.Enabled {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		return &Provider{}, func(_ context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel resource: %w", err)
	}

	conn, err := grpc.NewClient(cfg.OTLPEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("otel grpc conn: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, nil, fmt.Errorf("otel exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	p := &Provider{tp: tp}
	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	return p, shutdown, nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
