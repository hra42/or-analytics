# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OR Analytics is a cloud-native Go application that fetches activity data from the OpenRouter API and stores it incrementally in **DuckLake** - a lakehouse format combining data lakes and warehouses. The application uses in-memory DuckDB for processing and persists all data in S3-compatible object storage with automatic versioning and time travel.

The application tracks API usage, costs, token consumption, and model performance with efficient incremental appends (only new data written daily).

**Key Dependencies:**
- `github.com/hra42/openrouter-go` (v1.0.0) - OpenRouter API client
- `github.com/duckdb/duckdb-go/v2` (v2.5.0) - Official DuckDB database driver with DuckLake extensions
- `github.com/go-co-op/gocron/v2` (v2.17.0) - Scheduler library
- Go 1.26+

**Important:** This application requires an OpenRouter **provisioning key** (not a regular API key) from https://openrouter.ai/settings/provisioning-keys. Set it via `OPENROUTER_API_KEY` environment variable.

## Development Commands

### Running the Application

```bash
# Basic run (fetches last 30 days, incrementally appends)
go run main.go

# With specific date filter
go run main.go -date 2025-10-08

# With custom DuckLake database name
go run main.go -db my_analytics

# With verbose logging
go run main.go -verbose

# Custom PostgreSQL catalog
go run main.go \
  -pg-host 192.168.2.21 \
  -pg-port 5432 \
  -pg-user admin \
  -pg-dbname or_analytics_catalog

# Custom S3/R2 endpoint
go run main.go \
  -s3-endpoint s3.hra42.com \
  -s3-bucket my-analytics \
  -s3-region us-east-1
```

**Required Environment Variables:**
- `OPENROUTER_API_KEY` - OpenRouter provisioning key
- `PG_PASSWORD` - PostgreSQL catalog password
- `S3_KEY` - S3/R2 access key ID
- `S3_SECRET` - S3/R2 secret access key

**Command-line flags:**
- `-date` - Filter by specific date (YYYY-MM-DD)
- `-db` - DuckLake database name (default: or_analytics)
- `-verbose` - Enable verbose logging
- `-schedule` - Run as scheduler (daily, hourly, now, or cron expression)
- `-timezone` - Scheduler timezone (default: UTC)
- `-webhook-url` - Webhook URL for notifications
- `-pg-host`, `-pg-port`, `-pg-user`, `-pg-password`, `-pg-dbname` - PostgreSQL catalog config
- `-s3-key`, `-s3-secret`, `-s3-endpoint`, `-s3-bucket`, `-s3-region` - S3/R2 config

### Scheduler Mode

```bash
# Run as scheduler (daily at midnight UTC)
go run main.go -schedule daily

# Custom cron schedule (2 AM EST)
go run main.go -schedule "0 2 * * *" -timezone America/New_York

# Run now then schedule daily
go run main.go -schedule now

# With webhook notifications
go run main.go -schedule daily -webhook-url https://hooks.example.com/analytics
```

**Docker scheduler:**
```bash
# Start scheduler as daemon
docker-compose --profile scheduler up -d or-analytics-scheduler

# View logs
docker-compose logs -f or-analytics-scheduler
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
go test -v -run TestBuildPostgresConnStr
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
  -e OPENROUTER_API_KEY=sk-or-v1-... \
  -e PG_PASSWORD=... \
  -e S3_KEY=... \
  -e S3_SECRET=... \
  or-analytics -verbose

# Build and run specific platforms
docker buildx build --platform linux/amd64,linux/arm64 -t or-analytics .
```

### Querying DuckLake Data

```bash
# From DuckDB CLI
duckdb
```

```sql
-- Install extensions
INSTALL ducklake;
INSTALL postgres;
INSTALL httpfs;
INSTALL aws;

-- Configure credentials
CREATE SECRET s3_bucket (
    TYPE S3,
    KEY_ID 'your-key',
    SECRET 'your-secret',
    REGION 'us-east-1',
    ENDPOINT 's3.hra42.com',
    USE_SSL true,
    URL_STYLE 'path'
);

-- Attach DuckLake database
ATTACH 'ducklake:postgres:dbname=or_analytics_catalog host=192.168.2.21 port=5432 user=admin password=...'
AS or_analytics (DATA_PATH 's3://or-analytics');

USE or_analytics;

-- Query data
SELECT * FROM analytics LIMIT 10;

-- Time travel
SELECT * FROM analytics AS OF TIMESTAMP '2025-10-01 00:00:00';
```

## Architecture

### File Structure
The codebase follows a flat structure with clear separation of concerns:

- **main.go** - CLI entry point, mode selection (one-time vs scheduler), DuckLake config
- **scheduler.go** - Built-in scheduler using gocron/v2
- **ducklake.go** - DuckLake operations (connect, incremental append, queries)
- **processor.go** - Data transformation (API response → ActivityRecord format)
- **webhook.go** - Webhook notification handler
- **\*_test.go** - Comprehensive unit tests for each module
- **Dockerfile** - Multi-stage build for optimized container images
- **docker-compose.yml** - Docker Compose configuration with profiles
- **.dockerignore** - Excludes unnecessary files from Docker build context

### Data Flow

1. **Fetch**: `main.go` uses `openrouter-go` client to fetch last 30 days of activity from OpenRouter API
2. **Transform**: `processor.go` converts API response (`openrouter.ActivityData`) to internal `ActivityRecord` format
3. **Check Last Date**: `ducklake.go` queries `MAX(date)` from DuckLake to determine what data already exists
4. **Filter**: Only records newer than the last stored date are selected for append
5. **Append**: `ducklake.go` performs incremental INSERT into DuckLake (creates new snapshot automatically)
6. **Persist**: DuckLake writes Parquet files to S3 and updates PostgreSQL catalog
7. **Display**: `main.go` queries summary statistics from DuckLake and prints to console

### Database Schema

The `analytics` table in DuckLake uses a composite primary key `(date, model, provider_name)` to ensure uniqueness.

```sql
CREATE TABLE analytics (
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

**In-Memory Processing**: Application uses in-memory DuckDB (no local persistence) - stateless and eliminates single point of failure (`ducklake.go:69-74`).

**Incremental Append**: Only inserts records with dates newer than `MAX(date)` from DuckLake - prevents rewriting 42 days of data daily (`ducklake.go:91-95`, `main.go:183-191`).

**Automatic Snapshots**: DuckLake creates a new version/snapshot with each INSERT - enables time travel queries without additional code.

**NULL Handling**: Summary queries use `sql.NullFloat64` to handle empty tables correctly (`ducklake.go:250-270`).

**Date Normalization**: Dates are truncated to YYYY-MM-DD format to ensure consistent storage (`processor.go:8-14`).

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

Tests use helper functions and configuration validation. Key test coverage:
- PostgreSQL connection string building
- DuckLake configuration structure
- Date normalization
- Data transformation (API → ActivityRecord)
- Webhook payload generation

**Note:** Full DuckLake integration tests require:
- Running PostgreSQL instance for catalog
- S3/R2 credentials and bucket access
- DuckLake extensions installed in DuckDB

These are better suited for CI/CD or manual testing environments with proper credentials configured.

## S3-Compatible Services

The application supports any S3-compatible object storage service:

**Supported Services:**
- **AWS S3** - Default
- **Cloudflare R2** - Use custom `-s3-endpoint`
- **MinIO** - Use custom `-s3-endpoint`
- **DigitalOcean Spaces** - Use custom `-s3-endpoint`
- **Backblaze B2** - Use custom `-s3-endpoint`
- **Wasabi** - Use custom `-s3-endpoint`

**Configuration:**
- `DuckLakeConfig` struct in `ducklake.go:35-45` holds all S3 settings
- Credentials passed to DuckDB via `CREATE SECRET` SQL command
- Endpoint, bucket, and region are configurable via CLI flags or environment variables

## Docker Deployment

### Dockerfile Design

The project uses a **multi-stage build** for optimal image size and security:

**Stage 1: Builder**
- Based on `golang:1.26` (Debian-based)
- Installs build dependencies (gcc for CGO)
- Compiles with optimizations (`-ldflags="-s -w" -trimpath`)
- CGO_ENABLED=1 required for DuckDB
- **Note**: Uses Debian instead of Alpine to avoid DuckDB compilation issues

**Stage 2: Runtime**
- Based on `debian:trixie-slim` (minimal footprint)
- Includes ca-certificates, tzdata, and libstdc++6
- Runs as non-root user (analytics:1000)
- **No local database needed** - all persistence in DuckLake/S3
- **Note**: Debian Trixie is required for glibc 2.38+ which DuckDB needs at runtime

### Docker Compose Profiles

Service configurations available:

1. **Scheduler** (`or-analytics-scheduler`): Long-running scheduler (daily at midnight)

Use with: `docker-compose --profile scheduler up -d or-analytics-scheduler`

### Environment Variables

**Required:**
- `OPENROUTER_API_KEY` - OpenRouter provisioning key
- `PG_PASSWORD` - PostgreSQL catalog password
- `S3_KEY` - S3/R2 access key ID
- `S3_SECRET` - S3/R2 secret access key

**Optional:**
- `PG_HOST`, `PG_PORT`, `PG_USER`, `PG_DBNAME` - PostgreSQL catalog config
- `S3_ENDPOINT`, `S3_BUCKET`, `S3_REGION` - S3/R2 config

### No Volume Mounts Needed

Unlike traditional databases, DuckLake mode requires **no local volumes** - all data is persisted in S3 via DuckLake. The application runs stateless.

## Common Tasks

**Adding a new field to the schema:**
1. Update `ActivityRecord` struct in `ducklake.go`
2. Update the DuckLake table schema (if creating fresh database)
3. Update `ConvertActivityData()` in `processor.go`
4. Add corresponding tests in `ducklake_test.go`

**Creating new analysis queries:**
Add SQL files to `queries/` directory and document in `queries/README.md`. All queries should use DuckLake connection pattern shown above.

**Modifying API client behavior:**
The OpenRouter client is imported from `github.com/hra42/openrouter-go` - modifications should be made in that separate repository.

**Migrating from old local database mode:**
If you have existing data in the old `analytics.db` file:
1. Set up DuckLake infrastructure (PostgreSQL catalog + S3 bucket)
2. Export old data: `duckdb analytics.db -c "COPY analytics TO 'export.parquet' (FORMAT PARQUET)"`
3. Import to DuckLake: Connect via DuckDB CLI and run `INSERT INTO analytics SELECT * FROM read_parquet('export.parquet')`
