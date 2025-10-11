package main

import (
	"database/sql"
	"fmt"
)

const (
	createTable = `
		CREATE TABLE IF NOT EXISTS activity (
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
	`
	upsertQuery = `
		INSERT INTO activity (
			date, model, provider_name, requests, usage,
			prompt_tokens, completion_tokens, reasoning_tokens, byok_usage_inference, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, now())
		ON CONFLICT (date, model, provider_name)
		DO UPDATE SET
			requests = EXCLUDED.requests,
			usage = EXCLUDED.usage,
			prompt_tokens = EXCLUDED.prompt_tokens,
			completion_tokens = EXCLUDED.completion_tokens,
			reasoning_tokens = EXCLUDED.reasoning_tokens,
			byok_usage_inference = EXCLUDED.byok_usage_inference,
			created_at = now();
	`
	summaryQuery = `
		SELECT
			COUNT(*) as total_records,
			COUNT(DISTINCT date) as unique_dates,
			COUNT(DISTINCT model) as unique_models,
			COUNT(DISTINCT provider_name) as unique_providers,
			SUM(requests) as total_requests,
			SUM(usage) as total_usage,
			SUM(prompt_tokens) as total_prompt_tokens,
			SUM(completion_tokens) as total_completion_tokens,
			SUM(reasoning_tokens) as total_reasoning_tokens
		FROM activity;
	`
)

// Summary contains aggregated statistics from the activity table
type Summary struct {
	TotalRecords          int
	UniqueDates           int
	UniqueModels          int
	UniqueProviders       int
	TotalRequests         float64
	TotalUsage            float64
	TotalPromptTokens     float64
	TotalCompletionTokens float64
	TotalReasoningTokens  float64
}

// ActivityRecord represents a single activity record to be inserted
type ActivityRecord struct {
	Date               string
	Model              string
	ProviderName       string
	Requests           float64
	Usage              float64
	PromptTokens       float64
	CompletionTokens   float64
	ReasoningTokens    float64
	BYOKUsageInference float64
}

// InitDB opens a database connection and creates the activity table
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec(createTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

// InsertActivityRecords inserts or updates multiple activity records in a transaction
func InsertActivityRecords(db *sql.DB, records []ActivityRecord) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(upsertQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, record := range records {
		_, err := stmt.Exec(
			record.Date,
			record.Model,
			record.ProviderName,
			record.Requests,
			record.Usage,
			record.PromptTokens,
			record.CompletionTokens,
			record.ReasoningTokens,
			record.BYOKUsageInference,
		)
		if err != nil {
			return inserted, fmt.Errorf("failed to insert record for %s/%s: %w", record.Date, record.Model, err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return inserted, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return inserted, nil
}

// GetSummary retrieves aggregated statistics from the activity table
func GetSummary(db *sql.DB) (*Summary, error) {
	var summary Summary
	var totalRequests, totalUsage, totalPromptTokens, totalCompletionTokens, totalReasoningTokens sql.NullFloat64

	err := db.QueryRow(summaryQuery).Scan(
		&summary.TotalRecords,
		&summary.UniqueDates,
		&summary.UniqueModels,
		&summary.UniqueProviders,
		&totalRequests,
		&totalUsage,
		&totalPromptTokens,
		&totalCompletionTokens,
		&totalReasoningTokens,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary: %w", err)
	}

	// Handle NULL values (when table is empty)
	if totalRequests.Valid {
		summary.TotalRequests = totalRequests.Float64
	}
	if totalUsage.Valid {
		summary.TotalUsage = totalUsage.Float64
	}
	if totalPromptTokens.Valid {
		summary.TotalPromptTokens = totalPromptTokens.Float64
	}
	if totalCompletionTokens.Valid {
		summary.TotalCompletionTokens = totalCompletionTokens.Float64
	}
	if totalReasoningTokens.Valid {
		summary.TotalReasoningTokens = totalReasoningTokens.Float64
	}

	return &summary, nil
}

// ExportToParquet exports all activity data to a Parquet file
func ExportToParquet(db *sql.DB, outputPath string) error {
	// Use DuckDB's COPY command to export to Parquet
	query := fmt.Sprintf("COPY activity TO '%s' (FORMAT PARQUET)", outputPath)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to export to parquet: %w", err)
	}
	return nil
}
