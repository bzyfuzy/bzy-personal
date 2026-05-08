#!/usr/bin/env bash
set -euo pipefail

export PATH="$(go env GOPATH)/bin:${PATH}"

echo "▶ Setting up bzy-personal development environment..."

# ── Go tooling ────────────────────────────────────────────────────────────────
echo "▶ Installing Go tools..."
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# ── buf (proto toolchain) ─────────────────────────────────────────────────────
if ! command -v buf &>/dev/null; then
  echo "▶ Installing buf..."
  BIN_DIR="${HOME}/.local/bin"
  mkdir -p "${BIN_DIR}"
  BUF_VERSION="1.47.2"
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)
  [[ "${ARCH}" == "x86_64" ]] && ARCH="x86_64"
  [[ "${ARCH}" == "aarch64" || "${ARCH}" == "arm64" ]] && ARCH="arm64"
  curl -sSL \
    "https://github.com/bufbuild/buf/releases/download/v${BUF_VERSION}/buf-${OS}-${ARCH}" \
    -o "${BIN_DIR}/buf"
  chmod +x "${BIN_DIR}/buf"
fi

# ── Go workspace sync ─────────────────────────────────────────────────────────
echo "▶ Syncing Go workspace..."
go work sync

# ── Copy env files ────────────────────────────────────────────────────────────
echo "▶ Copying .env.example files..."
for dir in apps/bzy-brain apps/bzy-runner apps/bzy-interface; do
  if [[ -f "${dir}/.env.example" && ! -f "${dir}/.env" ]]; then
    cp "${dir}/.env.example" "${dir}/.env"
    echo "  Created ${dir}/.env"
  fi
done

# ── Start dev infra ───────────────────────────────────────────────────────────
echo "▶ Starting dev infrastructure (postgres, redis, nats, jaeger)..."
docker compose -f deploy/docker/docker-compose.dev.yml up -d

# ── Wait for postgres ─────────────────────────────────────────────────────────
echo "▶ Waiting for postgres..."
until docker compose -f deploy/docker/docker-compose.dev.yml exec postgres \
    pg_isready -U bzy 2>/dev/null; do
  sleep 1
done

# ── Create databases ──────────────────────────────────────────────────────────
echo "▶ Creating databases..."
for db in bzy_brain bzy_runner; do
  docker compose -f deploy/docker/docker-compose.dev.yml exec -T postgres \
    psql -U bzy -tc "SELECT 1 FROM pg_database WHERE datname='${db}'" \
    | grep -q 1 \
    || docker compose -f deploy/docker/docker-compose.dev.yml exec -T postgres \
         createdb -U bzy "${db}"
  echo "  ${db} ready"
done

# ── Run migrations ────────────────────────────────────────────────────────────
echo "▶ Running brain migrations..."
migrate -path migrations/brain \
  -database "postgres://bzy:bzy_dev@localhost:5432/bzy_brain?sslmode=disable" up

echo "▶ Running runner migrations..."
migrate -path migrations/runner \
  -database "postgres://bzy:bzy_dev@localhost:5432/bzy_runner?sslmode=disable" up

# ── Seed default data ─────────────────────────────────────────────────────────
echo "▶ Seeding default admin user..."
docker compose -f deploy/docker/docker-compose.dev.yml exec -T postgres \
  psql -U bzy -d bzy_brain < scripts/seed.sql

echo ""
echo "✅ Setup complete!"
echo ""
echo "  Default credentials:  admin@bzy.local / admin"
echo "  ⚠️  Change the admin password before exposing to any network."
echo ""
echo "  Run services:"
echo "    make run-bzy-brain     → gRPC :50051"
echo "    make run-bzy-runner    → worker pool"
echo "    make run-bzy-interface → HTTP :8080"
echo ""
echo "  Observability:"
echo "    Jaeger UI:  http://localhost:16686"
echo "    NATS:       http://localhost:8222"
