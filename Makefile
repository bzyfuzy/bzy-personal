SHELL := /bin/bash
.DEFAULT_GOAL := help

# ── Config ─────────────────────────────────────────────────────────────────────
MODULE       := github.com/bzyfuzy/bzy-personal
GO           := go
GOFLAGS      :=
BIN_DIR      := bin
PROTO_DIR    := proto
GEN_DIR      := gen
DOCKER_REPO  := ghcr.io/bzyfuzy

# ── Services ───────────────────────────────────────────────────────────────────
SERVICES := bzy-brain bzy-runner bzy-interface

# ── Tooling ────────────────────────────────────────────────────────────────────
PROTOC         := protoc
PROTOC_GEN_GO  := protoc-gen-go
PROTOC_GEN_GRPC := protoc-gen-go-grpc
GOLANGCI_LINT  := golangci-lint
MIGRATE        := migrate

# ── Build ──────────────────────────────────────────────────────────────────────
.PHONY: build
build: $(addprefix build-,$(SERVICES)) ## Build all service binaries

.PHONY: build-%
build-%:
	@echo "▶ Building $*..."
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BIN_DIR)/$* ./apps/$*/cmd/...

.PHONY: run-%
run-%: ## Run a service locally (e.g. make run-bzy-brain)
	$(GO) run ./apps/$*/cmd/...

# ── Test ───────────────────────────────────────────────────────────────────────
.PHONY: test
test: ## Run all tests
	$(GO) test ./... -race -cover -timeout 120s

.PHONY: test-%
test-%: ## Run tests for a specific service
	$(GO) test ./apps/$*/... -race -cover -timeout 60s

.PHONY: test-integration
test-integration: ## Run integration tests (requires docker compose up)
	$(GO) test ./... -tags=integration -race -timeout 300s

.PHONY: coverage
coverage: ## Generate HTML coverage report
	$(GO) test ./... -coverprofile=coverage.txt -covermode=atomic
	$(GO) tool cover -html=coverage.txt -o coverage.html

# ── Lint & Format ──────────────────────────────────────────────────────────────
.PHONY: lint
lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

.PHONY: fmt
fmt: ## Format all Go code
	$(GO) fmt ./...
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Tidy all go.mod files
	@for dir in pkg apps/bzy-brain apps/bzy-runner apps/bzy-interface; do \
		echo "▶ Tidying $$dir..."; \
		(cd $$dir && $(GO) mod tidy); \
	done
	$(GO) work sync

# ── Proto ──────────────────────────────────────────────────────────────────────
.PHONY: proto
proto: ## Generate protobuf code
	@mkdir -p $(GEN_DIR)
	@find $(PROTO_DIR) -name '*.proto' | while read f; do \
		$(PROTOC) \
			--go_out=$(GEN_DIR) \
			--go_opt=paths=source_relative \
			--go-grpc_out=$(GEN_DIR) \
			--go-grpc_opt=paths=source_relative \
			-I $(PROTO_DIR) $$f; \
	done
	@echo "✓ Protobuf generation complete"

# ── Database ───────────────────────────────────────────────────────────────────
.PHONY: migrate-brain
migrate-brain: ## Run brain database migrations
	$(MIGRATE) -path migrations/brain -database "$(BRAIN_DB_URL)" up

.PHONY: migrate-runner
migrate-runner: ## Run runner database migrations
	$(MIGRATE) -path migrations/runner -database "$(RUNNER_DB_URL)" up

.PHONY: migrate-down-%
migrate-down-%: ## Roll back one migration for a service
	$(MIGRATE) -path migrations/$* -database "$($*_DB_URL)" down 1

# ── Docker ─────────────────────────────────────────────────────────────────────
.PHONY: docker-build
docker-build: $(addprefix docker-build-,$(SERVICES)) ## Build all Docker images

.PHONY: docker-build-%
docker-build-%:
	@echo "▶ Building Docker image for $*..."
	docker build \
		--build-arg SERVICE=$* \
		-f apps/$*/Dockerfile \
		-t $(DOCKER_REPO)/$*:latest \
		-t $(DOCKER_REPO)/$*:$(shell git rev-parse --short HEAD) \
		.

.PHONY: docker-push-%
docker-push-%:
	docker push $(DOCKER_REPO)/$*:latest
	docker push $(DOCKER_REPO)/$*:$(shell git rev-parse --short HEAD)

.PHONY: up
up: ## Start all services via Docker Compose
	docker compose -f deploy/docker/docker-compose.yml up -d

.PHONY: down
down: ## Stop all services
	docker compose -f deploy/docker/docker-compose.yml down

.PHONY: dev
dev: ## Start infrastructure only (postgres, redis, nats) for local dev
	docker compose -f deploy/docker/docker-compose.dev.yml up -d

.PHONY: dev-ui
dev-ui: ## Start the Vite dev server for bzy-ui
	npm --prefix apps/bzy-ui install --prefer-offline
	npm --prefix apps/bzy-ui run dev

.PHONY: dev-all
dev-all: dev ## Start infra + all Go services + UI (Ctrl-C stops all)
	@echo "Starting services..."
	@trap 'kill 0' INT; \
	  (cd apps/bzy-brain     && go run ./cmd/) & \
	  (cd apps/bzy-runner    && go run ./cmd/) & \
	  (cd apps/bzy-interface && go run ./cmd/) & \
	  (npm --prefix apps/bzy-ui run dev)       & \
	  wait

.PHONY: dev-down
dev-down: ## Stop dev infrastructure
	docker compose -f deploy/docker/docker-compose.dev.yml down -v

.PHONY: logs-%
logs-%: ## Tail logs for a service (e.g. make logs-bzy-brain)
	docker compose -f deploy/docker/docker-compose.yml logs -f $*

# ── Kubernetes ─────────────────────────────────────────────────────────────────
.PHONY: k8s-apply
k8s-apply: ## Apply base k8s manifests
	kubectl apply -k deploy/k8s/base/

.PHONY: k8s-delete
k8s-delete: ## Delete k8s manifests
	kubectl delete -k deploy/k8s/base/

# ── Setup ──────────────────────────────────────────────────────────────────────
.PHONY: setup
setup: ## Bootstrap dev environment (install tools, migrate db, install npm deps)
	@bash scripts/setup.sh
	npm --prefix apps/bzy-ui install

.PHONY: seed
seed: ## Re-run the dev seed (idempotent — safe to run multiple times)
	docker compose -f deploy/docker/docker-compose.dev.yml exec -T postgres \
	  psql -U bzy -d bzy_brain < scripts/seed.sql
	@echo "Seeded: admin@bzy.local / admin"

.PHONY: env
env: ## Copy .env.example files
	@for dir in apps/bzy-brain apps/bzy-runner apps/bzy-interface; do \
		[ -f $$dir/.env.example ] && cp -n $$dir/.env.example $$dir/.env || true; \
	done

# ── CI Helpers ─────────────────────────────────────────────────────────────────
.PHONY: ci
ci: fmt lint test ## Run full CI pipeline locally

.PHONY: check
check: ## Quick pre-commit check
	$(GO) build ./...
	$(GO) vet ./...

# ── Help ───────────────────────────────────────────────────────────────────────
.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_%-]+:.*##' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'
