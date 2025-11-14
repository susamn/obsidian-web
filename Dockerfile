# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled (required for SQLite)
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o obsidian-web cmd/server/main.go

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite wget

WORKDIR /app

# Create necessary directories
RUN mkdir -p /data/indexes /etc/obsidian-web

# Copy binary from build stage
COPY --from=builder /app/obsidian-web .

# Copy example config (for reference)
COPY config.example.yaml /etc/obsidian-web/config.example.yaml

# Set default config path
ENV OBSIDIAN_WEB_CONFIG_PATH=/etc/obsidian-web/config.yaml

# Expose default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=40s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:8080/health || exit 1

# Run as non-root user
RUN addgroup -g 1000 obsidian && \
    adduser -D -u 1000 -G obsidian obsidian && \
    chown -R obsidian:obsidian /app /data

USER obsidian

CMD ["./obsidian-web"]
