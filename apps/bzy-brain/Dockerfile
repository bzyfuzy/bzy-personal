# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

# Copy go workspace files
COPY go.work go.work.sum* ./
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY apps/bzy-brain/go.mod apps/bzy-brain/go.sum* ./apps/bzy-brain/

# Pre-fetch deps (layer-cached until go.mod changes)
RUN cd apps/bzy-brain && go mod download

# Copy full source
COPY pkg/ ./pkg/
COPY apps/bzy-brain/ ./apps/bzy-brain/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -X main.version=$(git describe --tags --always 2>/dev/null || echo dev)" \
    -o /out/bzy-brain \
    ./apps/bzy-brain/cmd/...

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/bzy-brain /bzy-brain
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

EXPOSE 50051 9090

ENTRYPOINT ["/bzy-brain"]
