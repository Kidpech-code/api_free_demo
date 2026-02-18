# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.25.1-alpine AS builder

# Install CA certificates and git (needed for go mod download)
RUN apk add --no-cache ca-certificates git tzdata

WORKDIR /build

# Cache dependencies separately from source code
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a fully static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /app/server ./cmd/api/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM scratch

# Copy CA certs and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the static binary
COPY --from=builder /app/server /app/server

# Expose the application port (Railway uses the PORT env var)
EXPOSE 8080

ENTRYPOINT ["/app/server"]
