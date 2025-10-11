package main

import (
	"os"
	"testing"

	_ "github.com/marcboeker/go-duckdb"
)

func TestInitDB(t *testing.T) {
	dbPath := "test_init.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Verify we can query the table
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM activity;").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query activity table: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 records in new database, got %d", count)
	}
}

func TestInitDB_InvalidPath(t *testing.T) {
	// Try to create database in non-existent directory
	dbPath := "/nonexistent/directory/test.db"

	db, err := InitDB(dbPath)
	if err == nil {
		db.Close()
		t.Fatal("Expected error for invalid path, got nil")
	}
}

func TestInsertActivityRecords_Empty(t *testing.T) {
	dbPath := "test_empty_insert.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	count, err := InsertActivityRecords(db, []ActivityRecord{})
	if err != nil {
		t.Errorf("InsertActivityRecords with empty slice should not error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 inserted records, got %d", count)
	}
}

func TestInsertActivityRecords_Single(t *testing.T) {
	dbPath := "test_single_insert.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	records := []ActivityRecord{
		{
			Date:               "2025-10-09",
			Model:              "test/model",
			ProviderName:       "test-provider",
			Requests:           10.0,
			Usage:              0.5,
			PromptTokens:       1000.0,
			CompletionTokens:   500.0,
			ReasoningTokens:    100.0,
			BYOKUsageInference: 0.0,
		},
	}

	count, err := InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("InsertActivityRecords failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 inserted record, got %d", count)
	}

	// Verify record exists
	var dbCount int
	err = db.QueryRow("SELECT COUNT(*) FROM activity;").Scan(&dbCount)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}
	if dbCount != 1 {
		t.Errorf("Expected 1 record in database, got %d", dbCount)
	}
}

func TestInsertActivityRecords_Multiple(t *testing.T) {
	dbPath := "test_multiple_insert.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	records := []ActivityRecord{
		{
			Date:             "2025-10-09",
			Model:            "model1",
			ProviderName:     "provider1",
			Requests:         10.0,
			Usage:            0.5,
			PromptTokens:     1000.0,
			CompletionTokens: 500.0,
		},
		{
			Date:             "2025-10-09",
			Model:            "model2",
			ProviderName:     "provider1",
			Requests:         5.0,
			Usage:            0.3,
			PromptTokens:     500.0,
			CompletionTokens: 250.0,
		},
		{
			Date:             "2025-10-10",
			Model:            "model1",
			ProviderName:     "provider2",
			Requests:         8.0,
			Usage:            0.4,
			PromptTokens:     800.0,
			CompletionTokens: 400.0,
		},
	}

	count, err := InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("InsertActivityRecords failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 inserted records, got %d", count)
	}
}

func TestInsertActivityRecords_Upsert(t *testing.T) {
	dbPath := "test_upsert.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Insert first record
	records := []ActivityRecord{
		{
			Date:             "2025-10-09",
			Model:            "test/model",
			ProviderName:     "test-provider",
			Requests:         10.0,
			Usage:            0.5,
			PromptTokens:     1000.0,
			CompletionTokens: 500.0,
		},
	}

	count, err := InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("First insert failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 inserted record, got %d", count)
	}

	// Update same record (upsert)
	records[0].Usage = 0.75
	records[0].Requests = 15.0

	count, err = InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 upserted record, got %d", count)
	}

	// Verify only one record exists with updated values
	var dbCount int
	var usage, requests float64
	err = db.QueryRow("SELECT COUNT(*) FROM activity;").Scan(&dbCount)
	if err != nil {
		t.Fatalf("Failed to query count: %v", err)
	}
	if dbCount != 1 {
		t.Errorf("Expected 1 record in database, got %d", dbCount)
	}

	err = db.QueryRow("SELECT usage, requests FROM activity;").Scan(&usage, &requests)
	if err != nil {
		t.Fatalf("Failed to query values: %v", err)
	}
	if usage != 0.75 {
		t.Errorf("Expected usage 0.75, got %f", usage)
	}
	if requests != 15.0 {
		t.Errorf("Expected requests 15.0, got %f", requests)
	}
}

func TestGetSummary_Empty(t *testing.T) {
	dbPath := "test_summary_empty.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	summary, err := GetSummary(db)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.TotalRecords != 0 {
		t.Errorf("Expected 0 total records, got %d", summary.TotalRecords)
	}
}

func TestGetSummary_WithData(t *testing.T) {
	dbPath := "test_summary_data.db"
	defer os.Remove(dbPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	records := []ActivityRecord{
		{
			Date:             "2025-10-01",
			Model:            "model1",
			ProviderName:     "provider1",
			Requests:         10.0,
			Usage:            0.5,
			PromptTokens:     1000.0,
			CompletionTokens: 500.0,
			ReasoningTokens:  100.0,
		},
		{
			Date:             "2025-10-01",
			Model:            "model2",
			ProviderName:     "provider1",
			Requests:         5.0,
			Usage:            0.3,
			PromptTokens:     800.0,
			CompletionTokens: 400.0,
			ReasoningTokens:  50.0,
		},
		{
			Date:             "2025-10-02",
			Model:            "model1",
			ProviderName:     "provider2",
			Requests:         8.0,
			Usage:            0.4,
			PromptTokens:     900.0,
			CompletionTokens: 450.0,
			ReasoningTokens:  75.0,
		},
	}

	_, err = InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("InsertActivityRecords failed: %v", err)
	}

	summary, err := GetSummary(db)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.TotalRecords != 3 {
		t.Errorf("Expected 3 total records, got %d", summary.TotalRecords)
	}
	if summary.UniqueDates != 2 {
		t.Errorf("Expected 2 unique dates, got %d", summary.UniqueDates)
	}
	if summary.UniqueModels != 2 {
		t.Errorf("Expected 2 unique models, got %d", summary.UniqueModels)
	}
	if summary.UniqueProviders != 2 {
		t.Errorf("Expected 2 unique providers, got %d", summary.UniqueProviders)
	}
	if summary.TotalRequests != 23.0 {
		t.Errorf("Expected 23.0 total requests, got %f", summary.TotalRequests)
	}

	expectedUsage := 1.2
	tolerance := 0.000001
	if summary.TotalUsage < expectedUsage-tolerance || summary.TotalUsage > expectedUsage+tolerance {
		t.Errorf("Expected ~1.2 total usage, got %f", summary.TotalUsage)
	}

	if summary.TotalPromptTokens != 2700.0 {
		t.Errorf("Expected 2700.0 total prompt tokens, got %f", summary.TotalPromptTokens)
	}
	if summary.TotalCompletionTokens != 1350.0 {
		t.Errorf("Expected 1350.0 total completion tokens, got %f", summary.TotalCompletionTokens)
	}
	if summary.TotalReasoningTokens != 225.0 {
		t.Errorf("Expected 225.0 total reasoning tokens, got %f", summary.TotalReasoningTokens)
	}
}

func TestExportToParquet(t *testing.T) {
	dbPath := "test_export.db"
	parquetPath := "test_export.parquet"
	defer os.Remove(dbPath)
	defer os.Remove(parquetPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Insert test data
	records := []ActivityRecord{
		{
			Date:             "2025-10-01",
			Model:            "model1",
			ProviderName:     "provider1",
			Requests:         10.0,
			Usage:            0.5,
			PromptTokens:     1000.0,
			CompletionTokens: 500.0,
			ReasoningTokens:  100.0,
		},
		{
			Date:             "2025-10-02",
			Model:            "model2",
			ProviderName:     "provider2",
			Requests:         5.0,
			Usage:            0.3,
			PromptTokens:     800.0,
			CompletionTokens: 400.0,
			ReasoningTokens:  50.0,
		},
	}

	_, err = InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("InsertActivityRecords failed: %v", err)
	}

	// Export to Parquet
	err = ExportToParquet(db, parquetPath)
	if err != nil {
		t.Fatalf("ExportToParquet failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(parquetPath); os.IsNotExist(err) {
		t.Errorf("Parquet file was not created at %s", parquetPath)
	}

	// Verify we can read it back with DuckDB
	var count int
	query := "SELECT COUNT(*) FROM read_parquet('" + parquetPath + "')"
	err = db.QueryRow(query).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to read parquet file: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 records in parquet file, got %d", count)
	}
}

func TestExportToParquet_EmptyTable(t *testing.T) {
	dbPath := "test_export_empty.db"
	parquetPath := "test_export_empty.parquet"
	defer os.Remove(dbPath)
	defer os.Remove(parquetPath)

	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	// Export empty table
	err = ExportToParquet(db, parquetPath)
	if err != nil {
		t.Fatalf("ExportToParquet failed on empty table: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(parquetPath); os.IsNotExist(err) {
		t.Errorf("Parquet file was not created at %s", parquetPath)
	}
}
