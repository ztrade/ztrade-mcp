# ====== Build Stage ======
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Dependency cache layer
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.Version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o /ztrade-mcp .

# ====== Runtime Stage ======
# Go plugin (.so) requires the exact same Go version at runtime as at build time,
# so we use the same golang base image instead of a minimal alpine.
FROM golang:1.25-alpine

RUN apk add --no-cache ca-certificates tzdata gcc musl-dev

COPY --from=builder /ztrade-mcp /usr/local/bin/ztrade-mcp

# Default config and data directories
RUN mkdir -p /etc/ztrade /data/ztrade

VOLUME ["/etc/ztrade", "/data/ztrade"]

# Default HTTP port
EXPOSE 8080

ENTRYPOINT ["ztrade-mcp"]
CMD ["--config", "/etc/ztrade/ztrade.yaml", "--transport", "http", "--listen", ":8080"]
