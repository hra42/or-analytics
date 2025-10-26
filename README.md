# OR Analytics

A cloud-native analytics solution for OpenRouter API usage data using **DuckLake** for incremental, versioned data storage.

## Overview

OR Analytics fetches activity data from the OpenRouter API and stores it incrementally in DuckLake - a lakehouse format that combines the benefits of data lakes and warehouses. Data is stored in S3-compatible object storage with automatic versioning and time travel capabilities.

**Key Features:**
- ✅ **Incremental Appends**: Only new data is written daily (no rewrites)
- ✅ **No Local Storage**: Uses in-memory DuckDB (stateless, no single point of failure)
- ✅ **Cloud-Native**: All data persisted in S3/R2 via DuckLake
- ✅ **Version Control**: Automatic snapshots for time travel queries
- ✅ **Built-in Scheduler**: Run as a daily cron job or custom schedule

## Architecture

```
OpenRouter API → DuckDB (in-memory) → DuckLake → PostgreSQL Catalog + S3/R2 Storage
```

1. **Fetch**: Retrieve last 30 days of activity from OpenRouter
2. **Filter**: Only select records newer than the last stored date
3. **Append**: Write new records to DuckLake (creates new snapshot)
4. **Persist**: Data automatically saved to S3, metadata in PostgreSQL

## Prerequisites

- Go 1.25.1+
- OpenRouter **provisioning key** (not regular API key): https://openrouter.ai/settings/provisioning-keys
- PostgreSQL instance for DuckLake catalog
- S3-compatible object storage (AWS S3, Cloudflare R2, MinIO, etc.)

### Key Dependencies

- [`github.com/duckdb/duckdb-go/v2`](https://github.com/duckdb/duckdb-go) (v2.5.0) - Official DuckDB Go driver with DuckLake extensions
- [`github.com/hra42/openrouter-go`](https://github.com/hra42/openrouter-go) (v1.0.0) - OpenRouter API client
- [`github.com/go-co-op/gocron/v2`](https://github.com/go-co-op/gocron) (v2.17.0) - Scheduler library

## Quick Start

### 1. Set up DuckLake Infrastructure

First, configure your DuckLake catalog and storage:

```sql
-- In your PostgreSQL instance
CREATE DATABASE or_analytics_catalog;
```

Your S3/R2 bucket should be created and accessible with access keys.

### 2. Configure Environment Variables

```bash
# Required: OpenRouter API key
export OPENROUTER_API_KEY="sk-or-v1-..."

# Required: PostgreSQL catalog credentials
export PG_PASSWORD="your-postgres-password"

# Required: S3/R2 credentials
export S3_KEY="your-s3-access-key"
export S3_SECRET="your-s3-secret-key"
```

### 3. Run One-Time Import

```bash
# Basic import (uses default configuration)
go run main.go

# With custom configuration
go run main.go \
  -db my_analytics \
  -pg-host 192.168.2.21 \
  -pg-port 5432 \
  -s3-endpoint s3.hra42.com \
  -s3-bucket my-analytics \
  -verbose
```

### 4. Run as Scheduler

```bash
# Daily at midnight (UTC)
go run main.go -schedule daily

# Custom schedule (2 AM EST)
go run main.go -schedule "0 2 * * *" -timezone America/New_York

# Run now, then schedule daily
go run main.go -schedule now

# With webhook notifications
go run main.go -schedule daily -webhook-url https://hooks.example.com/analytics
```

## Configuration Flags

### Required (via flags or env vars)
- `-pg-password` / `PG_PASSWORD` - PostgreSQL catalog password
- `-s3-key` / `S3_KEY` - S3/R2 access key ID
- `-s3-secret` / `S3_SECRET` - S3/R2 secret access key

### Database & Catalog
- `-db` - DuckLake database name (default: `or_analytics`)
- `-pg-host` - PostgreSQL host (default: `192.168.2.21`)
- `-pg-port` - PostgreSQL port (default: `5432`)
- `-pg-user` - PostgreSQL user (default: `admin`)
- `-pg-dbname` - PostgreSQL catalog database (default: `or_analytics_catalog`)

### S3 Storage
- `-s3-endpoint` - S3/R2 endpoint URL (default: `s3.hra42.com`)
- `-s3-bucket` - S3/R2 bucket name (default: `or-analytics`)
- `-s3-region` - S3/R2 region (default: `us-east-1`)

### Scheduler
- `-schedule` - Schedule mode: `daily`, `hourly`, `now`, or cron expression
- `-timezone` - Timezone for scheduler (default: `UTC`)
- `-webhook-url` - Webhook URL for notifications
- `-date` - Filter by specific date (YYYY-MM-DD)
- `-verbose` - Enable verbose logging

## Docker Deployment

### Build Image

```bash
docker build -t or-analytics .
```

### Run One-Time

```bash
docker run --rm \
  -e OPENROUTER_API_KEY=sk-or-v1-... \
  -e PG_PASSWORD=... \
  -e S3_KEY=... \
  -e S3_SECRET=... \
  or-analytics
```

### Run as Scheduler

```bash
docker run -d \
  --name or-analytics-scheduler \
  -e OPENROUTER_API_KEY=sk-or-v1-... \
  -e PG_PASSWORD=... \
  -e S3_KEY=... \
  -e S3_SECRET=... \
  or-analytics -schedule daily -verbose
```

### Docker Compose

```bash
# Start scheduler
docker-compose --profile scheduler up -d

# View logs
docker-compose logs -f or-analytics-scheduler
```

## Querying Data

Connect to your DuckLake database from any DuckDB client:

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

-- Attach database
ATTACH 'ducklake:postgres:dbname=or_analytics_catalog host=192.168.2.21 port=5432 user=admin password=...'
AS or_analytics (DATA_PATH 's3://or-analytics');

USE or_analytics;

-- Query your data
SELECT
    date,
    model,
    SUM(requests) as total_requests,
    SUM(usage) as total_cost
FROM activity
WHERE date >= current_date - 7
GROUP BY date, model
ORDER BY date DESC, total_cost DESC;
```

### Time Travel

DuckLake automatically creates snapshots, allowing you to query historical data:

```sql
-- Query data as it was at a specific timestamp
SELECT * FROM activity AS OF TIMESTAMP '2025-10-01 00:00:00';

-- Query a specific snapshot version
SELECT * FROM activity AS OF VERSION 42;
```

## Development

### Run Tests

```bash
# All tests
go test -v

# With coverage
go test -v -cover

# With race detection
go test -v -race
```

### Build Optimized Binary

```bash
CGO_ENABLED=1 go build -ldflags="-s -w" -trimpath -o or-analytics
```

## How It Works

### Incremental Append Pattern

Unlike traditional approaches that rewrite all data daily, OR Analytics uses an efficient incremental pattern:

1. **Query Last Date**: Check `MAX(date)` in DuckLake
2. **Filter API Results**: Only process records newer than last date
3. **Append New Data**: Insert only new records (e.g., 1 day vs 42 days)
4. **Auto-Snapshot**: DuckLake creates new version automatically

**Example:**
- Day 1: Import 30 days of history → 30 days written
- Day 2: Import last 30 days, append 1 new day → 1 day written ✅
- Day 3: Import last 30 days, append 1 new day → 1 day written ✅

### Why DuckLake?

- **No Local Database Needed**: Stateless, runs anywhere
- **Incremental Writes**: Only new data, no rewrites
- **Automatic Versioning**: Time travel built-in
- **S3-Native**: Data lives in cheap object storage
- **SQL Queries**: Full DuckDB analytics capabilities

## Troubleshooting

### Authentication Errors

If you get 401/403 errors:
- Ensure you're using a **provisioning key**, not a regular API key
- Get one at: https://openrouter.ai/settings/provisioning-keys

### Connection Issues

- **PostgreSQL**: Verify catalog host/port and credentials
- **S3/R2**: Check endpoint URL, bucket name, and access keys
- **Firewall**: Ensure outbound access to PostgreSQL and S3 endpoints

### No New Data

This is normal if you've already imported today's data. The incremental append will skip duplicates.

## License

MIT

## Contributing

Issues and pull requests welcome at https://github.com/hra42/or-analytics
