module github.com/bzyfuzy/bzy-personal/apps/bzy-brain

go 1.23

require (
	github.com/bzyfuzy/bzy-personal/pkg v0.0.0
	github.com/jackc/pgx/v5 v5.7.1
	github.com/pgvector/pgvector-go v0.2.2
	github.com/redis/go-redis/v9 v9.6.1
	github.com/google/uuid v1.6.0
	github.com/spf13/viper v1.19.0
	go.uber.org/fx v1.23.0
	go.uber.org/zap v1.27.0
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
	google.golang.org/grpc v1.68.0
	google.golang.org/protobuf v1.35.2
	github.com/golang-migrate/migrate/v4 v4.18.1
	github.com/sashabaranov/go-openai v1.32.5
	github.com/nats-io/nats.go v1.37.0
)

replace github.com/bzyfuzy/bzy-personal/pkg => ../../pkg
