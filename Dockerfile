# Multi-stage build for OR Analytics
# Stage 1: Build the Go application
FROM golang:1.25.1-alpine AS builder

# Install build dependencies (gcc, musl-dev for CGO)
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./

# Build the application with optimizations
# CGO_ENABLED=1 is required for DuckDB
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w" \
    -trimpath \
    -o or-analytics

# Stage 2: Create minimal runtime image
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 analytics && \
    adduser -D -u 1000 -G analytics analytics

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/or-analytics /app/or-analytics

# Create directory for database and set permissions
RUN mkdir -p /app/data && \
    chown -R analytics:analytics /app

# Switch to non-root user
USER analytics

# Set default database path to data directory
ENV DB_PATH=/app/data/analytics.db

# Entry point
ENTRYPOINT ["/app/or-analytics"]

# Default command (can be overridden)
CMD ["-db", "/app/data/analytics.db"]
