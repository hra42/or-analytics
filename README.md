# OR Analytics

OpenRouter Analytics Engine - Fetch and analyze your OpenRouter API usage data using DuckDB.

## Overview

OR Analytics is a lightweight tool that fetches activity data from OpenRouter's Activity API and stores it in a local DuckDB database for analysis. Track your API usage, costs, token consumption, and model performance over time.

## Features

- 📊 Fetch activity data from OpenRouter API (last 30 days)
- 💾 Store data in efficient DuckDB database
- 🔍 Filter by specific dates
- 📈 Built-in summary statistics
- 🔄 Upsert logic to prevent duplicates
- 📝 Example SQL queries for common analyses
- 📦 Export data to Parquet format (local or S3)
- ☁️ Direct S3 upload support (AWS S3, MinIO, DigitalOcean Spaces, Cloudflare R2, etc.)
- 🐳 Docker support with optimized multi-stage builds
- ⚡ Fast and lightweight

## Prerequisites

**For Docker (Recommended):**
- Docker and Docker Compose
- OpenRouter provisioning key (not a regular API key!)
  - Create one at: https://openrouter.ai/settings/provisioning-keys

**For Native Installation:**
- Go 1.25.1 or higher
- OpenRouter provisioning key (not a regular API key!)
  - Create one at: https://openrouter.ai/settings/provisioning-keys
- DuckDB CLI (optional, for running queries)

**Optional (for S3 export):**
- AWS credentials (or compatible service credentials)
  - Configure via `~/.aws/credentials` or environment variables
  - Required environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION`

## Installation

### Quick Start with Docker (Recommended)

The fastest way to get started:

```bash
# Clone the repository
git clone https://github.com/hra42/or-analytics.git
cd or-analytics

# Set up environment
cp .env.example .env
# Edit .env and add your OPENROUTER_API_KEY

# Create data directory
mkdir -p data

# Run with Docker Compose
docker-compose run --rm or-analytics
```

That's it! Your analytics data is now in `./data/analytics.db`.

### Native Installation

For development or if you prefer running natively:

1. Clone the repository:
```bash
git clone https://github.com/hra42/or-analytics.git
cd or-analytics
```

2. Install dependencies:
```bash
go mod download
```

3. Set up your environment:
```bash
cp .env.example .env
# Edit .env and add your OpenRouter provisioning key
export OPENROUTER_API_KEY=your_provisioning_key_here
```

## Usage

### Basic Usage

Fetch all activity data (last 30 completed UTC days):
```bash
go run main.go
```

### Command-Line Options

```bash
# Filter by specific date
go run main.go -date 2025-10-08

# Use custom database path
go run main.go -db /path/to/custom.db

# Enable verbose logging
go run main.go -verbose

# Export data to local Parquet file
go run main.go -export activity.parquet

# Export data directly to AWS S3
go run main.go -export s3://my-bucket/analytics/activity.parquet

# Export to S3-compatible service (MinIO)
go run main.go -export s3://my-bucket/data.parquet \
  -s3-endpoint https://minio.example.com:9000 \
  -s3-path-style

# Combine options
go run main.go -date 2025-10-08 -db custom.db -verbose -export s3://my-bucket/data.parquet
```

**Available Flags:**
- `-date` - Filter by specific date (YYYY-MM-DD format)
- `-db` - Path to DuckDB database file (default: analytics.db)
- `-verbose` - Enable verbose logging
- `-export` - Export data to Parquet file (local path or s3:// URI)
- `-s3-endpoint` - Custom S3 endpoint URL for S3-compatible services
- `-s3-path-style` - Use path-style S3 URLs (required for MinIO and some providers)
- `-schedule` - Run as a scheduler with specified schedule (daily, hourly, now, or cron expression)
- `-timezone` - Timezone for scheduler (default: UTC)

### Example Output

```
OpenRouter Analytics - Activity Data Importer
==============================================

Fetching all activity data (last 30 completed UTC days)
Retrieved 185 activity records
✓ Successfully imported 185 records

Database Summary
================
Total records in database: 185
Date range: 30 unique dates
Models used: 19 unique models
Providers: 16 unique providers

Total API requests: 2706
Total usage cost: $3.9100
Total tokens:
  Prompt: 16786827
  Completion: 1314629
  Reasoning: 575490

Database saved to: analytics.db
```

## Database Schema

The `activity` table has the following structure:

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

## Analyzing Data

### Using SQL Queries

The `queries/` directory contains example SQL queries. Run them using DuckDB CLI:

```bash
# Install DuckDB CLI (macOS)
brew install duckdb

# Run a query
duckdb analytics.db < queries/01_summary_stats.sql

# Interactive mode
duckdb analytics.db
D SELECT * FROM activity LIMIT 5;
```

### Example Queries

- `01_summary_stats.sql` - Overall statistics
- `02_top_models_by_spend.sql` - Most expensive models
- `03_provider_distribution.sql` - Provider cost breakdown
- `04_daily_spend_trend.sql` - Daily spending trends
- `05_model_provider_breakdown.sql` - Model-provider combinations
- `06_recent_activity.sql` - Last 7 days activity
- `07_cost_efficiency.sql` - Cost per 1K tokens
- `08_weekly_summary.sql` - Weekly spending summary

See [queries/README.md](queries/README.md) for more details.

### Exporting Data

You can export your data in multiple formats:

#### Using the built-in export flag (recommended)

**Local Parquet files:**
```bash
# Export to local Parquet file during import
go run main.go -export activity.parquet

# Export specific date range
go run main.go -date 2025-10-08 -export oct08_activity.parquet
```

**Direct S3 upload:**
```bash
# Export directly to AWS S3 (requires AWS credentials)
go run main.go -export s3://my-bucket/analytics/activity.parquet

# Export to S3 with date in filename
go run main.go -export s3://my-bucket/analytics/$(date +%Y-%m-%d).parquet

# Export specific date to S3
go run main.go -date 2025-10-08 -export s3://my-bucket/daily/2025-10-08.parquet
```

**S3-Compatible Providers (MinIO, DigitalOcean Spaces, Cloudflare R2, etc.):**
```bash
# MinIO
go run main.go -export s3://my-bucket/data.parquet \
  -s3-endpoint https://minio.example.com:9000 \
  -s3-path-style

# DigitalOcean Spaces
go run main.go -export s3://my-space/analytics/data.parquet \
  -s3-endpoint https://nyc3.digitaloceanspaces.com

# Cloudflare R2
go run main.go -export s3://my-bucket/data.parquet \
  -s3-endpoint https://[account-id].r2.cloudflarestorage.com

# Backblaze B2
go run main.go -export s3://my-bucket/data.parquet \
  -s3-endpoint https://s3.us-west-001.backblazeb2.com

# Wasabi
go run main.go -export s3://my-bucket/data.parquet \
  -s3-endpoint https://s3.wasabisys.com
```

**AWS Credentials Setup:**
```bash
# Option 1: Environment variables
export AWS_ACCESS_KEY_ID=your_access_key
export AWS_SECRET_ACCESS_KEY=your_secret_key
export AWS_REGION=us-east-1

# Option 2: AWS credentials file (~/.aws/credentials)
[default]
aws_access_key_id = your_access_key
aws_secret_access_key = your_secret_key
region = us-east-1
```

#### Using DuckDB CLI

Export to CSV:
```bash
duckdb analytics.db "COPY (SELECT * FROM activity) TO 'export.csv' (HEADER, DELIMITER ',');"
```

Export to Parquet:
```bash
duckdb analytics.db "COPY (SELECT * FROM activity) TO 'export.parquet' (FORMAT PARQUET);"
```

Export filtered data:
```bash
duckdb analytics.db "COPY (SELECT * FROM activity WHERE date >= '2025-10-01') TO 'october.parquet' (FORMAT PARQUET);"
```

## Scheduler (Built-in)

The application includes a built-in scheduler that eliminates the need for external cron jobs. The scheduler runs continuously and executes imports on a defined schedule.

### Usage

**Daily at midnight (default):**
```bash
# Run scheduler
go run main.go -schedule daily

# With export to S3
go run main.go -schedule daily -export s3://my-bucket/analytics.parquet

# With custom timezone
go run main.go -schedule daily -timezone America/New_York
```

**Hourly:**
```bash
go run main.go -schedule hourly -verbose
```

**Custom cron expression:**
```bash
# Every day at 2 AM
go run main.go -schedule "0 2 * * *"

# Every 6 hours
go run main.go -schedule "0 */6 * * *"

# Monday-Friday at 9 AM
go run main.go -schedule "0 9 * * 1-5"
```

**Run now and schedule:**
```bash
# Run immediately, then daily at midnight
go run main.go -schedule now
```

### Docker Scheduler

Run the scheduler as a long-running container:

```bash
# Basic scheduler (daily at midnight UTC)
docker-compose --profile scheduler up -d or-analytics-scheduler

# With S3 export
docker-compose --profile scheduler-s3 up -d or-analytics-scheduler-s3

# View logs
docker-compose logs -f or-analytics-scheduler

# Stop scheduler
docker-compose stop or-analytics-scheduler
```

### Scheduler Benefits

- ✅ **No external dependencies**: No need for cron, systemd timers, or Kubernetes CronJobs
- ✅ **Timezone support**: Run schedules in any timezone
- ✅ **Flexible scheduling**: Daily, hourly, or custom cron expressions
- ✅ **Self-contained**: Everything runs in one process
- ✅ **Docker-friendly**: Runs as a long-running container with restart policies
- ✅ **Graceful shutdown**: Handles SIGTERM and SIGINT properly

## Automation (External Cron)

Alternatively, you can use external scheduling tools to run the tool periodically:

### Using cron (Linux/macOS)

**Basic daily import:**
```bash
# Edit crontab
crontab -e

# Add entry to run daily at 2 AM
0 2 * * * cd /path/to/or-analytics && /usr/local/go/bin/go run main.go >> logs/import.log 2>&1
```

**Daily import with S3 backup:**
```bash
# Edit crontab
crontab -e

# Run daily at 2 AM and upload to S3
0 2 * * * cd /path/to/or-analytics && /usr/local/go/bin/go run main.go -export s3://my-bucket/backups/$(date +\%Y-\%m-\%d).parquet >> logs/import.log 2>&1
```

**With Docker:**
```bash
# Edit crontab
crontab -e

# Run daily at 2 AM using Docker
0 2 * * * cd /path/to/or-analytics && docker-compose run --rm or-analytics -export s3://my-bucket/backups/analytics-$(date +\%Y-\%m-\%d).parquet >> logs/import.log 2>&1
```

### Using systemd timer (Linux)

Create `/etc/systemd/system/or-analytics.service`:
```ini
[Unit]
Description=OpenRouter Analytics Import

[Service]
Type=oneshot
WorkingDirectory=/path/to/or-analytics
Environment="OPENROUTER_API_KEY=your_key_here"
ExecStart=/usr/local/go/bin/go run main.go
```

Create `/etc/systemd/system/or-analytics.timer`:
```ini
[Unit]
Description=OpenRouter Analytics Timer

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

Enable and start:
```bash
sudo systemctl enable or-analytics.timer
sudo systemctl start or-analytics.timer
```

### Using Kubernetes CronJob

For Kubernetes deployments:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: or-analytics
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: or-analytics
            image: or-analytics:latest
            env:
            - name: OPENROUTER_API_KEY
              valueFrom:
                secretKeyRef:
                  name: or-analytics-secrets
                  key: openrouter-api-key
            - name: AWS_ACCESS_KEY_ID
              valueFrom:
                secretKeyRef:
                  name: aws-credentials
                  key: access-key-id
            - name: AWS_SECRET_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: aws-credentials
                  key: secret-access-key
            - name: AWS_REGION
              value: "us-east-1"
            args:
            - "-export"
            - "s3://my-bucket/analytics/data.parquet"
            volumeMounts:
            - name: data
              mountPath: /app/data
          restartPolicy: OnFailure
          volumes:
          - name: data
            persistentVolumeClaim:
              claimName: or-analytics-data
```

## Testing

Run the test suite:

```bash
# Run all tests
go test -v

# Run tests with coverage
go test -v -cover

# Run tests with race detection
go test -v -race
```

The project includes comprehensive unit tests covering:
- Database schema validation
- Upsert functionality
- Summary query accuracy
- Primary key constraints
- Date parsing
- NULL value handling
- Timestamp creation

## Building

### Native Binary

Build a standalone binary:

```bash
go build -o or-analytics

# Run the binary
./or-analytics
```

### Docker

Build and run with Docker:

```bash
# Build the Docker image
docker build -t or-analytics .

# Run with Docker
docker run --rm \
  -e OPENROUTER_API_KEY=your_key_here \
  -v $(pwd)/data:/app/data \
  or-analytics

# Run with specific options
docker run --rm \
  -e OPENROUTER_API_KEY=your_key_here \
  -v $(pwd)/data:/app/data \
  or-analytics -date 2025-10-08 -verbose
```

### Docker Compose

The easiest way to run with Docker:

```bash
# Copy environment file
cp .env.example .env
# Edit .env and add your OPENROUTER_API_KEY

# Run once
docker-compose run --rm or-analytics

# Run with custom options
docker-compose run --rm or-analytics -date 2025-10-08 -verbose

# Export to S3
docker-compose run --rm or-analytics \
  -export s3://my-bucket/analytics.parquet

# Export to MinIO (uses profile)
docker-compose run --rm or-analytics-minio
```

**Docker Compose Profiles:**
- Default: Basic import without export
- `scheduled`: Automated S3 backup
- `minio`: Export to MinIO-compatible storage

```bash
# Create data directory for persistence
mkdir -p data

# Run default service
docker-compose run --rm or-analytics

# Run with scheduled profile
docker-compose --profile scheduled run --rm or-analytics-scheduled

# Run with MinIO profile
docker-compose --profile minio run --rm or-analytics-minio
```

## CI/CD

The project uses Woodpecker CI for automated testing, building, and deployment.

### Pipeline Triggers

The pipeline runs automatically on:
- **Push to main**: Runs tests, builds binaries, uploads to S3, builds and pushes Docker images
- **Pull requests**: Runs tests and dry-run Docker builds
- **Tags**: Builds and pushes Docker images with version tags
- **Manual triggers**: All steps can be triggered manually from the Woodpecker UI

### Automated Steps

1. **Testing**: Runs Go tests with race detection and coverage
2. **Binary Builds**: Builds for linux/amd64 and linux/arm64
3. **S3 Upload**: Uploads binaries to S3 (main branch and manual triggers)
4. **Docker Images**: Builds multi-platform Docker images (linux/amd64, linux/arm64)
5. **Notifications**: Sends Discord notifications on completion

### Required Secrets

Configure these secrets in Woodpecker:
- `s3_bucket`: S3 bucket name for binary uploads
- `s3_access_key`: S3 access key
- `s3_secret_key`: S3 secret key
- `s3_region`: S3 region
- `s3_endpoint`: S3 endpoint URL
- `docker_registry`: Container registry URL (e.g., registry.hra42.com)
- `docker_repo`: Docker repository name (e.g., or-analytics)
- `discord_webhook_id`: Discord webhook ID for notifications
- `discord_webhook_token`: Discord webhook token for notifications

**Note**: The Docker registry configuration assumes no authentication is required. If your registry requires authentication, you'll need to add `username` and `password` settings to the docker-buildx steps.

## Common Use Cases

### Cost Analysis
Track spending trends and identify expensive models or providers.

### Usage Monitoring
Monitor API usage patterns and optimize based on actual consumption.

### Budget Tracking
Set up alerts or reports based on spending thresholds.

### Model Comparison
Compare different models' performance and cost efficiency.

### Provider Analysis
Understand which providers you're using most and their costs.

## Troubleshooting

### "OPENROUTER_API_KEY environment variable is required"
Make sure you've exported the environment variable or set it in your shell.

### "This endpoint requires a provisioning key"
You need a provisioning key, not a regular inference API key. Create one at:
https://openrouter.ai/settings/provisioning-keys

### No activity data found
This is normal for new accounts or if you haven't used the API in the last 30 days.

### Database locked errors
Make sure only one instance is writing to the database at a time.

### S3 export errors

**"failed to load AWS config"**
- Ensure AWS credentials are configured via `~/.aws/credentials` or environment variables
- Check that `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_REGION` are set

**"invalid S3 URI"**
- S3 URIs must be in the format `s3://bucket-name/path/to/file.parquet`
- Ensure the bucket name and key are separated by a `/`

**"failed to upload to S3: AccessDenied"**
- Verify your AWS credentials have write permissions to the S3 bucket
- Check the bucket policy and IAM permissions

### S3-compatible service connection issues

**MinIO connection problems:**
- Always use `-s3-path-style` flag with MinIO
- Ensure the endpoint URL includes the protocol (https:// or http://)
- Example: `-s3-endpoint https://minio.example.com:9000 -s3-path-style`

**DigitalOcean Spaces / Cloudflare R2:**
- These services typically don't require `-s3-path-style`
- Ensure you're using the correct regional endpoint
- Credentials should be configured the same way as AWS S3

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [OpenRouter](https://openrouter.ai/) for the excellent API service
- [DuckDB](https://duckdb.org/) for the powerful embedded database
- [openrouter-go](https://github.com/hra42/openrouter-go) for the Go client library

## Related Projects

- [openrouter-go](https://github.com/hra42/openrouter-go) - Go client for OpenRouter API

## Development

For developers working on this project, see [CLAUDE.md](CLAUDE.md) for development guidelines, architecture overview, and common tasks.

## Support

For issues and questions:
- Open an issue on GitHub
- Check OpenRouter documentation: https://openrouter.ai/docs
