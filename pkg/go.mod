module github.com/bzyfuzy/bzy-personal/pkg

go 1.23

require (
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.28.0
	go.opentelemetry.io/otel/sdk v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
	go.opentelemetry.io/otel/metric v1.28.0
	go.opentelemetry.io/otel/sdk/metric v1.28.0
	go.opentelemetry.io/otel/exporters/prometheus v0.50.0
	go.uber.org/zap v1.27.0
	github.com/prometheus/client_golang v1.20.0
	github.com/google/uuid v1.6.0
	github.com/spf13/viper v1.19.0
	github.com/redis/go-redis/v9 v9.6.1
	github.com/nats-io/nats.go v1.37.0
	github.com/gorilla/websocket v1.5.3
)
