# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /workspace

COPY go.work go.work.sum* ./
COPY pkg/go.mod pkg/go.sum* ./pkg/
COPY apps/bzy-interface/go.mod apps/bzy-interface/go.sum* ./apps/bzy-interface/

RUN cd apps/bzy-interface && go mod download

COPY pkg/ ./pkg/
COPY apps/bzy-interface/ ./apps/bzy-interface/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" \
    -o /out/bzy-interface \
    ./apps/bzy-interface/cmd/...

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/bzy-interface /bzy-interface
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

EXPOSE 8080

ENTRYPOINT ["/bzy-interface"]
