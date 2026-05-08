module github.com/bzyfuzy/bzy-personal/apps/bzy-interface

go 1.23

require (
	github.com/bzyfuzy/bzy-personal/pkg v0.0.0
	github.com/gin-gonic/gin v1.10.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/redis/go-redis/v9 v9.6.1
	github.com/spf13/viper v1.19.0
	go.uber.org/fx v1.23.0
	go.uber.org/zap v1.27.0
	go.opentelemetry.io/otel v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.56.0
	google.golang.org/grpc v1.68.0
	google.golang.org/protobuf v1.35.2
	github.com/nats-io/nats.go v1.37.0
	golang.org/x/time v0.8.0
)

replace github.com/bzyfuzy/bzy-personal/pkg => ../../pkg
