package main

import (
	"database/sql"
	"fmt"
	"strings"
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

// DuckLakeConfig holds configuration for DuckLake integration
type DuckLakeConfig struct {
	Enabled         bool   // Enable DuckLake mode
	PostgresConnStr string // PostgreSQL connection string (e.g., "dbname=catalog host=localhost port=5432 user=admin password=...")
	DatabaseName    string // DuckLake database name (e.g., "or_analytics")
	S3AccessKey     string // S3/R2 access key ID
	S3SecretKey     string // S3/R2 secret access key
	S3Endpoint      string // S3/R2 endpoint URL (e.g., "s3.example.com")
	S3Bucket        string // S3/R2 bucket name (e.g., "or-analytics")
	S3Region        string // S3/R2 region (default: "us-east-1")
}

const (
	// DuckLake setup commands
	installExtensions = `
		INSTALL ducklake;
		INSTALL postgres;
		INSTALL httpfs;
		INSTALL aws;
	`

	createSecret = `
		CREATE OR REPLACE SECRET s3_bucket (
			TYPE S3,
			KEY_ID ?,
			SECRET ?,
			REGION ?,
			ENDPOINT ?,
			USE_SSL true,
			URL_STYLE 'path'
		);
	`

	attachDatabase = `
		ATTACH 'ducklake:postgres:%s' AS %s (DATA_PATH 's3://%s');
	`

	useDatabase = `USE %s;`

	// Query to get the maximum date already in the DuckLake table
	getMaxDateQuery = `
		SELECT MAX(date) as max_date FROM %s.analytics;
	`

	// Insert new data that doesn't exist in DuckLake yet
	incrementalInsert = `
		INSERT INTO %s.analytics
		SELECT * FROM local_activity
		WHERE date > COALESCE((SELECT MAX(date) FROM %s.analytics), '1900-01-01');
	`
)

// InitDuckLake initializes an in-memory DuckDB connection with DuckLake extensions and attaches to the remote catalog
// Note: DuckLake mode uses in-memory database since all persistence is handled by DuckLake in S3
func InitDuckLake(config *DuckLakeConfig) (*sql.DB, error) {
	// Open in-memory DuckDB connection (no local persistence needed)
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to open in-memory database: %w", err)
	}

	// Install required extensions
	if _, err := db.Exec(installExtensions); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to install DuckLake extensions: %w", err)
	}

	// Create S3 secret
	if _, err := db.Exec(createSecret,
		config.S3AccessKey,
		config.S3SecretKey,
		config.S3Region,
		config.S3Endpoint,
	); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create S3 secret: %w", err)
	}

	// Attach DuckLake database
	attachQuery := fmt.Sprintf(attachDatabase, config.PostgresConnStr, config.DatabaseName, config.S3Bucket)
	if _, err := db.Exec(attachQuery); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to attach DuckLake database: %w", err)
	}

	// Switch to DuckLake database
	useQuery := fmt.Sprintf(useDatabase, config.DatabaseName)
	if _, err := db.Exec(useQuery); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to switch to DuckLake database: %w", err)
	}

	return db, nil
}

// GetLastDuckLakeDate retrieves the maximum date already stored in DuckLake
func GetLastDuckLakeDate(db *sql.DB, dbName string) (string, error) {
	query := fmt.Sprintf(getMaxDateQuery, dbName)

	var maxDate sql.NullString
	err := db.QueryRow(query).Scan(&maxDate)
	if err != nil {
		return "", fmt.Errorf("failed to get max date from DuckLake: %w", err)
	}

	if !maxDate.Valid || maxDate.String == "" {
		// No data exists yet, return empty string
		return "", nil
	}

	return maxDate.String, nil
}

// AppendToDuckLake performs an incremental append of new records to DuckLake
// It only inserts records with dates newer than what's already stored
func AppendToDuckLake(db *sql.DB, dbName string, records []ActivityRecord) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}

	// First, we need to ensure there's a local temp table with the schema
	// Create the local activity table structure (without inserting to it yet)
	localTableDDL := `
		CREATE TEMP TABLE IF NOT EXISTS local_activity (
			date DATE,
			model VARCHAR,
			provider_name VARCHAR,
			requests DOUBLE,
			usage DOUBLE,
			prompt_tokens DOUBLE,
			completion_tokens DOUBLE,
			reasoning_tokens DOUBLE,
			byok_usage_inference DOUBLE,
			created_at TIMESTAMP DEFAULT now()
		);
	`
	if _, err := db.Exec(localTableDDL); err != nil {
		return 0, fmt.Errorf("failed to create local temp table: %w", err)
	}

	// Clear any existing temp data
	if _, err := db.Exec("DELETE FROM local_activity"); err != nil {
		return 0, fmt.Errorf("failed to clear temp table: %w", err)
	}

	// Begin transaction for inserting records into temp table
	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert all fetched records into local temp table
	insertLocal := `
		INSERT INTO local_activity (
			date, model, provider_name, requests, usage,
			prompt_tokens, completion_tokens, reasoning_tokens, byok_usage_inference
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	stmt, err := tx.Prepare(insertLocal)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare insert statement: %w", err)
	}
	defer stmt.Close()

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
			return 0, fmt.Errorf("failed to insert record into temp table: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit temp data: %w", err)
	}

	// Now perform incremental insert from temp table to DuckLake
	// This only inserts records newer than what already exists
	insertQuery := fmt.Sprintf(incrementalInsert, dbName, dbName)

	result, err := db.Exec(insertQuery)
	if err != nil {
		return 0, fmt.Errorf("failed to append to DuckLake: %w", err)
	}

	// Get number of rows inserted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return int(rowsAffected), nil
}

// GetDuckLakeSummary retrieves summary statistics from DuckLake
func GetDuckLakeSummary(db *sql.DB, dbName string) (*Summary, error) {
	query := fmt.Sprintf(`
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
		FROM %s.analytics;
	`, dbName)

	var summary Summary
	var totalRequests, totalUsage, totalPromptTokens, totalCompletionTokens, totalReasoningTokens sql.NullFloat64

	err := db.QueryRow(query).Scan(
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
		return nil, fmt.Errorf("failed to get DuckLake summary: %w", err)
	}

	// Handle NULL values
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

// ParsePostgresConnStr parses a PostgreSQL connection string and returns individual components
// This is a helper for building the connection string from flags
func BuildPostgresConnStr(dbname, host, port, user, password string) string {
	var parts []string

	if dbname != "" {
		parts = append(parts, fmt.Sprintf("dbname=%s", dbname))
	}
	if host != "" {
		parts = append(parts, fmt.Sprintf("host=%s", host))
	}
	if port != "" {
		parts = append(parts, fmt.Sprintf("port=%s", port))
	}
	if user != "" {
		parts = append(parts, fmt.Sprintf("user=%s", user))
	}
	if password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", password))
	}

	return strings.Join(parts, " ")
}
