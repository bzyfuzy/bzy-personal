# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY apps/bzy-runner/go.mod apps/bzy-runner/go.sum* ./apps/bzy-runner/

RUN cd apps/bzy-runner && go mod download

COPY pkg/ ./pkg/
COPY apps/bzy-runner/ ./apps/bzy-runner/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/bzy-runner \
    ./apps/bzy-runner/cmd/...

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/bzy-runner /bzy-runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

EXPOSE 9091

ENTRYPOINT ["/bzy-runner"]
