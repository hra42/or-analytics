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
- ⚡ Fast and lightweight

## Prerequisites

- Go 1.21 or higher
- OpenRouter provisioning key (not a regular API key!)
  - Create one at: https://openrouter.ai/settings/provisioning-keys
- DuckDB CLI (optional, for running queries)

## Installation

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

# Combine options
go run main.go -date 2025-10-08 -db custom.db -verbose
```

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

Export to CSV:
```bash
duckdb analytics.db "COPY (SELECT * FROM activity) TO 'export.csv' (HEADER, DELIMITER ',');"
```

Export to Parquet:
```bash
duckdb analytics.db "COPY (SELECT * FROM activity) TO 'export.parquet' (FORMAT PARQUET);"
```

## Automation

Run the tool periodically to keep your database up to date:

### Using cron (Linux/macOS)

```bash
# Edit crontab
crontab -e

# Add entry to run daily at 2 AM
0 2 * * * cd /path/to/or-analytics && /usr/local/go/bin/go run main.go >> logs/import.log 2>&1
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

Build a standalone binary:

```bash
go build -o or-analytics main.go

# Run the binary
./or-analytics
```

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

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [OpenRouter](https://openrouter.ai/) for the excellent API service
- [DuckDB](https://duckdb.org/) for the powerful embedded database
- [openrouter-go](https://github.com/hra42/openrouter-go) for the Go client library

## Related Projects

- [openrouter-go](https://github.com/hra42/openrouter-go) - Go client for OpenRouter API

## Support

For issues and questions:
- Open an issue on GitHub
- Check OpenRouter documentation: https://openrouter.ai/docs
