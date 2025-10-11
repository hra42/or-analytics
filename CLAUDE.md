# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OR Analytics is a Go application that fetches activity data from the OpenRouter API and stores it in a DuckDB database for analysis. The application tracks API usage, costs, token consumption, and model performance.

**Key Dependencies:**
- `github.com/hra42/openrouter-go` (v1.0.0) - OpenRouter API client
- `github.com/marcboeker/go-duckdb` - DuckDB database driver
- `github.com/aws/aws-sdk-go-v2` - AWS SDK for S3 uploads
- `github.com/go-co-op/gocron/v2` - Scheduler library
- Go 1.25.1+

**Important:** This application requires an OpenRouter **provisioning key** (not a regular API key) from https://openrouter.ai/settings/provisioning-keys. Set it via `OPENROUTER_API_KEY` environment variable.

## Development Commands

### Running the Application
```bash
# Basic run (fetches last 30 days)
go run main.go

# With specific date filter
go run main.go -date 2025-10-08

# With custom database path
go run main.go -db custom.db

# With verbose logging
go run main.go -verbose

# Export to local Parquet file
go run main.go -export activity.parquet

# Export directly to AWS S3
go run main.go -export s3://bucket/path/file.parquet

# Export to S3-compatible service (MinIO)
go run main.go -export s3://bucket/file.parquet \
  -s3-endpoint https://minio.example.com:9000 \
  -s3-path-style

# Export to DigitalOcean Spaces
go run main.go -export s3://space/file.parquet \
  -s3-endpoint https://nyc3.digitaloceanspaces.com
```

**Command-line flags:**
- `-date` - Filter by specific date (YYYY-MM-DD)
- `-db` - Database path (default: analytics.db)
- `-verbose` - Enable verbose logging
- `-export` - Export path (local or s3://)
- `-s3-endpoint` - Custom S3 endpoint for S3-compatible services
- `-s3-path-style` - Use path-style URLs (required for MinIO)
- `-schedule` - Run as scheduler (daily, hourly, now, or cron expression)
- `-timezone` - Scheduler timezone (default: UTC)

### Scheduler Mode

```bash
# Run as scheduler (daily at midnight)
go run main.go -schedule daily

# Scheduler with S3 export
go run main.go -schedule daily -export s3://bucket/data.parquet

# Custom cron schedule
go run main.go -schedule "0 2 * * *" -timezone America/New_York

# Run now then schedule
go run main.go -schedule now
```

**Docker scheduler:**
```bash
# Start scheduler as daemon
docker-compose --profile scheduler up -d or-analytics-scheduler

# View logs
docker-compose logs -f or-analytics-scheduler
```
```

### Testing
```bash
# Run all tests
go test -v

# Run with coverage
go test -v -cover -coverprofile=coverage.out

# Run with race detection
go test -v -race

# Run specific test
go test -v -run TestInsertActivityRecords
```

### Building

**Native:**
```bash
# Standard build
go build -o or-analytics

# Optimized build (matching CI)
CGO_ENABLED=1 go build -ldflags="-s -w" -trimpath -o or-analytics
```

**Docker:**
```bash
# Build Docker image
docker build -t or-analytics .

# Run with Docker
docker run --rm \
  -e OPENROUTER_API_KEY=your_key \
  -v $(pwd)/data:/app/data \
  or-analytics

# Run with docker-compose
docker-compose run --rm or-analytics

# Build and run specific platforms
docker buildx build --platform linux/amd64,linux/arm64 -t or-analytics .
```

### Database Queries
```bash
# Run example queries
duckdb analytics.db < queries/01_summary_stats.sql

# Interactive mode
duckdb analytics.db
```

## Architecture

### File Structure
The codebase follows a flat structure with clear separation of concerns:

- **main.go** - CLI entry point, mode selection (one-time vs scheduler)
- **scheduler.go** - Built-in scheduler using gocron/v2
- **db.go** - Database operations (schema, upsert, queries, Parquet export)
- **s3.go** - S3 operations (upload, download, URI parsing)
- **processor.go** - Data transformation (API response → database records)
- **\*_test.go** - Comprehensive unit tests for each module
- **Dockerfile** - Multi-stage build for optimized container images
- **docker-compose.yml** - Docker Compose configuration with profiles
- **.dockerignore** - Excludes unnecessary files from Docker build context

### Data Flow
1. **Fetch**: `main.go` uses `openrouter-go` client to fetch activity data from OpenRouter API
2. **Transform**: `processor.go` converts API response (`openrouter.ActivityData`) to internal `ActivityRecord` format
3. **Store**: `db.go` upserts records into DuckDB using batch transactions (100 records per batch)
4. **Export** (optional): `db.go` exports to local Parquet, or `s3.go` uploads to S3
5. **Display**: `main.go` queries summary statistics and prints to console

### Database Schema
The `activity` table uses a composite primary key `(date, model, provider_name)` to ensure uniqueness. All upserts use `ON CONFLICT DO UPDATE` to handle duplicate imports gracefully.

```sql
CREATE TABLE activity (
    date DATE,
    model VARCHAR,
    provider_name VARCHAR,
    requests DOUBLE,
    usage DOUBLE,
    prompt_tokens DOUBLE,
    completion_tokens DOUBLE,
    reasoning_tokens DOUBLE,
    byok_usage_inference DOUBLE,
    created_at TIMESTAMP DEFAULT now(),
    PRIMARY KEY (date, model, provider_name)
);
```

### Key Design Patterns

**Batch Processing**: Records are inserted in batches of 100 using transactions for performance and consistency (see `main.go:92-109`).

**Upsert Logic**: All inserts use `ON CONFLICT DO UPDATE` to prevent duplicates and allow re-running without data duplication (see `db.go:24-39`).

**NULL Handling**: Summary queries use `sql.NullFloat64` to handle empty tables correctly (see `db.go:141-178`).

**Date Normalization**: Dates are truncated to YYYY-MM-DD format to ensure consistent storage (see `processor.go:8-14`).

## CI/CD

The project uses Woodpecker CI (`.woodpecker.yml`):
- Runs tests with race detection and coverage on push/PR
- Builds native binaries for both amd64 and arm64 architectures
- Uploads binary builds to S3 on main branch pushes
- Builds Docker images for linux/amd64 and linux/arm64
- Pushes Docker images to registry on main branch and tags
- Sends Discord notifications

### Docker Image Build Pipeline

**Pull Requests:**
- Dry-run build to validate Dockerfile
- Multi-platform build (amd64, arm64)
- Tagged with commit SHA and `latest`

**Main Branch:**
- Build and push to registry
- Tagged with: commit SHA (short), branch name, `latest`

**Tagged Releases:**
- Build and push to registry
- Tagged with: version tag, `latest`

### Required Secrets

For Docker builds, configure these secrets in Woodpecker CI:
- `docker_registry` - Docker registry URL (e.g., `ghcr.io` or `docker.io`)
- `docker_repo` - Full repository name (e.g., `hra42/or-analytics`)
- `docker_username` - Registry username
- `docker_password` - Registry password or token

## Testing Notes

Tests use in-memory DuckDB databases (`:memory:`) for isolation. Key test coverage:
- Schema validation and table creation
- Upsert behavior with duplicates
- Primary key constraint enforcement
- NULL value handling in summaries
- Date normalization
- Timestamp auto-generation
- Parquet export (local files)
- S3 URI parsing and validation

**Note:** Full S3 integration tests require AWS credentials and are better suited for CI/CD environments. Unit tests validate URI parsing and error handling without requiring actual S3 access.

## S3-Compatible Services

The application supports any S3-compatible object storage service through the `-s3-endpoint` and `-s3-path-style` flags:

**Supported Services:**
- **AWS S3** - Default, no additional flags needed
- **MinIO** - Use `-s3-endpoint` and `-s3-path-style`
- **DigitalOcean Spaces** - Use `-s3-endpoint`
- **Cloudflare R2** - Use `-s3-endpoint`
- **Backblaze B2** - Use `-s3-endpoint`
- **Wasabi** - Use `-s3-endpoint`

**Configuration:**
- `S3Config` struct in `s3.go:50-54` holds endpoint and path-style settings
- Custom endpoints are applied via AWS SDK v2's `BaseEndpoint` option
- Path-style addressing (bucket.s3.com vs s3.com/bucket) is controlled via `UsePathStyle` option

## Docker Deployment

### Dockerfile Design
The project uses a **multi-stage build** for optimal image size and security:

**Stage 1: Builder**
- Based on `golang:1.25.1-alpine`
- Installs build dependencies (gcc, musl-dev for CGO)
- Compiles with optimizations (`-ldflags="-s -w" -trimpath`)
- CGO_ENABLED=1 required for DuckDB

**Stage 2: Runtime**
- Based on `alpine:latest` (minimal footprint)
- Includes only ca-certificates and tzdata
- Runs as non-root user (analytics:1000)
- Database stored in `/app/data` for volume mounting

### Docker Compose Profiles
Service configurations available:

1. **Default** (`or-analytics`): One-time import, no export
2. **Scheduler** (`or-analytics-scheduler`): Long-running scheduler (daily at midnight)
3. **Scheduler-S3** (`or-analytics-scheduler-s3`): Scheduler with S3 export
4. **Scheduled** (`or-analytics-scheduled`): Legacy one-time S3 export
5. **MinIO** (`or-analytics-minio`): MinIO-compatible storage

Use profiles with: `docker-compose --profile <name> up -d <service>` (for schedulers) or `docker-compose --profile <name> run --rm <service>` (for one-time runs)

### Environment Variables
- `OPENROUTER_API_KEY` - Required for API access
- `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` - For S3 export
- `DB_PATH` - Database file path (default: `/app/data/analytics.db`)

### Volume Mounts
- `./data:/app/data` - Persistent database storage
- `~/.aws:/home/analytics/.aws:ro` - Optional: Host AWS credentials (read-only)

## Common Tasks

**Adding a new field to the schema:**
1. Update `ActivityRecord` struct in `db.go`
2. Update `createTable` constant in `db.go`
3. Update `upsertQuery` constant in `db.go`
4. Update `ConvertActivityData()` in `processor.go`
5. Add corresponding tests in `db_test.go`

**Creating new analysis queries:**
Add SQL files to `queries/` directory and document in `queries/README.md`.

**Modifying API client behavior:**
The OpenRouter client is imported from `github.com/hra42/openrouter-go` - modifications should be made in that separate repository.
