# bzy-personal

A self-hosted personal AI operating system — memory, reasoning, async execution, and multi-device orchestration.

## Services

| Service | Description | Port |
|---------|-------------|------|
| `bzy-brain` | AI memory, RAG, reasoning, agents, workflows | gRPC `:50051` |
| `bzy-runner` | Distributed task execution, scheduler, worker pool | gRPC `:50052` |
| `bzy-interface` | REST API, WebSocket, SSE streaming, auth gateway | HTTP `:8080` |

## Quick Start

```bash
# 1. Bootstrap dev environment (installs tools, starts infra, runs migrations)
make setup

# 2. Run services locally (each in a separate terminal)
make run-bzy-brain
make run-bzy-runner
make run-bzy-interface

# 3. Or run everything via Docker Compose
make up
```

## Development

```bash
make dev          # start postgres, redis, nats, jaeger only
make test         # run all unit tests
make lint         # golangci-lint
make build        # compile all service binaries to bin/
make proto        # regenerate protobuf code
make tidy         # go mod tidy all modules + sync workspace
make ci           # full CI pipeline (fmt + lint + test)
```

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full design documentation covering:

- Service communication (gRPC + event bus)
- AI memory architecture (episodic/semantic/working memory)
- RAG pipeline (ingest → chunk → embed → retrieve → augment → generate)
- Worker lifecycle and distributed execution model
- Redis vs NATS vs Kafka tradeoffs
- Authentication flow
- Streaming response design (SSE + WebSocket)
- Plugin SDK architecture
- MVP roadmap and scaling strategy

## Stack

| Layer | Technology |
|-------|-----------|
| Language | Go 1.23 |
| HTTP framework | Gin |
| Service mesh | gRPC |
| Database | PostgreSQL 17 + pgvector |
| Cache/Queue/Lock | Redis 7 |
| Message bus | Redis Streams → NATS JetStream |
| AI providers | OpenAI-compatible (OpenAI, Ollama, Groq, Together) |
| Tracing | OpenTelemetry → Jaeger |
| Metrics | Prometheus + Grafana |
| Logging | Zap (structured JSON) |
| DI | Uber fx |
| Migrations | golang-migrate |
| Container | Docker / Kubernetes + Kustomize |

## Environment Variables

Each service is configured via environment variables with a service-specific prefix:

```bash
# bzy-brain
BRAIN_AI_OPENAI_API_KEY=sk-...
BRAIN_DATABASE_DSN=postgres://bzy:pass@localhost:5432/bzy_brain
BRAIN_REDIS_ADDR=localhost:6379

# bzy-interface
INTERFACE_AUTH_JWT_SECRET=your-secret-here
INTERFACE_GATEWAY_BRAIN_ADDR=localhost:50051

# shared
*_LOG_LEVEL=debug|info|warn|error
*_TELEMETRY_ENABLED=true
*_TELEMETRY_OTLP_ENDPOINT=localhost:4317
```

Copy `.env.example` files: `make env`

## Project Structure

```
bzy-personal/
├── apps/bzy-brain/          # AI service
├── apps/bzy-runner/         # Execution service
├── apps/bzy-interface/      # API gateway
├── pkg/                     # Shared SDK
├── proto/                   # Protobuf contracts
├── migrations/              # SQL migrations
├── deploy/docker/           # Docker Compose
├── deploy/k8s/base/         # Kubernetes manifests
├── scripts/                 # Dev tooling
├── docs/                    # Architecture docs
├── go.work                  # Go workspace
└── Makefile                 # All commands
```
