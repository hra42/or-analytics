# Example SQL Queries

This directory contains example SQL queries for analyzing OpenRouter activity data stored in DuckDB.

## Running Queries

You can run these queries using the DuckDB CLI:

```bash
# Install DuckDB CLI if you haven't already
# macOS
brew install duckdb

# Or download from https://duckdb.org/docs/installation/

# Run a query
duckdb analytics.db < queries/01_summary_stats.sql

# Or use interactive mode
duckdb analytics.db
```

## Available Queries

### 01_summary_stats.sql
Overall summary statistics including total records, unique dates, models, providers, and aggregate costs.

### 02_top_models_by_spend.sql
Top 10 models ranked by total spend, with request counts and token usage.

### 03_provider_distribution.sql
Provider-level breakdown showing cost distribution and average costs per request.

### 04_daily_spend_trend.sql
Daily spending trends showing costs and usage over time.

### 05_model_provider_breakdown.sql
Detailed breakdown combining model and provider information.

### 06_recent_activity.sql
Recent activity from the last 7 days.

### 07_cost_efficiency.sql
Cost efficiency analysis showing cost per 1K tokens for different models.

### 08_weekly_summary.sql
Weekly aggregated spending and usage statistics.

## Custom Queries

You can also run custom queries directly:

```bash
duckdb analytics.db "SELECT model, SUM(usage) as total FROM analytics GROUP BY model;"
```

## Exporting Results

Export query results to CSV:

```bash
duckdb analytics.db "COPY (SELECT * FROM analytics) TO 'export.csv' (HEADER, DELIMITER ',');"
```

Export to Parquet for efficient storage:

```bash
duckdb analytics.db "COPY (SELECT * FROM analytics) TO 'export.parquet' (FORMAT PARQUET);"
```
