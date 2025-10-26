# Multi-stage build for OR Analytics
# Stage 1: Build the Go application
FROM registry.hra42.com/golang:1.25.1 AS builder

ENV GOPROXY=https://go.hra42.com
ENV GOSUMDB=sum.golang.org

# Install build dependencies (gcc for CGO and DuckDB)
RUN apt-get update && apt-get install -y gcc && rm -rf /var/lib/apt/lists/*

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
# Using trixie for glibc 2.38+ required by DuckDB
FROM debian:trixie-slim

# Install runtime dependencies
# libstdc++6 is required for DuckDB at runtime
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    libstdc++6 && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user with home directory
RUN groupadd -g 1000 analytics && \
    useradd -r -u 1000 -g analytics -m -d /home/analytics analytics

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/or-analytics /app/or-analytics

# Create directories and set permissions
RUN mkdir -p /app/data /home/analytics/.duckdb && \
    chown -R analytics:analytics /app /home/analytics

# Switch to non-root user
USER analytics

# Set default database path to data directory
ENV DB_PATH=/app/data/analytics.db

# Entry point
ENTRYPOINT ["/app/or-analytics"]

# Default command (can be overridden)
CMD ["-db", "/app/data/analytics.db"]
