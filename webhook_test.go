package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetDatabaseMetrics(t *testing.T) {
	// Create in-memory database
	db, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test with empty database
	metrics, err := GetDatabaseMetrics(db)
	if err != nil {
		t.Fatalf("Failed to get metrics from empty database: %v", err)
	}

	if metrics.TotalRecords != 0 {
		t.Errorf("Expected 0 total records, got %d", metrics.TotalRecords)
	}

	if metrics.UniqueDates != 0 {
		t.Errorf("Expected 0 unique dates, got %d", metrics.UniqueDates)
	}

	// Insert some test data
	records := []ActivityRecord{
		{
			Date:             "2025-10-01",
			Model:            "gpt-4",
			ProviderName:     "openai",
			Requests:         100,
			Usage:            1.50,
			PromptTokens:     1000,
			CompletionTokens: 500,
		},
		{
			Date:             "2025-10-02",
			Model:            "claude-3",
			ProviderName:     "anthropic",
			Requests:         50,
			Usage:            0.75,
			PromptTokens:     500,
			CompletionTokens: 250,
		},
	}

	_, err = InsertActivityRecords(db, records)
	if err != nil {
		t.Fatalf("Failed to insert test records: %v", err)
	}

	// Test with data
	metrics, err = GetDatabaseMetrics(db)
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}

	if metrics.TotalRecords != 2 {
		t.Errorf("Expected 2 total records, got %d", metrics.TotalRecords)
	}

	if metrics.UniqueDates != 2 {
		t.Errorf("Expected 2 unique dates, got %d", metrics.UniqueDates)
	}

	if metrics.UniqueModels != 2 {
		t.Errorf("Expected 2 unique models, got %d", metrics.UniqueModels)
	}

	if metrics.UniqueProviders != 2 {
		t.Errorf("Expected 2 unique providers, got %d", metrics.UniqueProviders)
	}

	if metrics.TotalRequests != 150 {
		t.Errorf("Expected 150 total requests, got %.0f", metrics.TotalRequests)
	}

	if metrics.TotalUsage != 2.25 {
		t.Errorf("Expected 2.25 total usage, got %.2f", metrics.TotalUsage)
	}

	if metrics.DateRangeStart != "2025-10-01" {
		t.Errorf("Expected start date 2025-10-01, got %s", metrics.DateRangeStart)
	}

	if metrics.DateRangeEnd != "2025-10-02" {
		t.Errorf("Expected end date 2025-10-02, got %s", metrics.DateRangeEnd)
	}

	if metrics.JobStatus != "success" {
		t.Errorf("Expected job status 'success', got %s", metrics.JobStatus)
	}
}

func TestSendWebhook(t *testing.T) {
	// Create test server
	var receivedPayload WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if r.Header.Get("User-Agent") != "OR-Analytics/1.0" {
			t.Errorf("Expected User-Agent OR-Analytics/1.0, got %s", r.Header.Get("User-Agent"))
		}

		// Decode payload
		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("Failed to decode webhook payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create test payload
	payload := &WebhookPayload{
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		TotalRecords:     100,
		UniqueDates:      30,
		UniqueModels:     5,
		UniqueProviders:  3,
		DateRangeStart:   "2025-09-01",
		DateRangeEnd:     "2025-09-30",
		TotalRequests:    1000,
		TotalUsage:       50.25,
		RecordsImported:  10,
		JobDuration:      "5s",
		JobStatus:        "success",
	}

	// Send webhook
	ctx := context.Background()
	err := SendWebhook(ctx, server.URL, payload)
	if err != nil {
		t.Fatalf("Failed to send webhook: %v", err)
	}

	// Verify received payload
	if receivedPayload.TotalRecords != payload.TotalRecords {
		t.Errorf("Expected %d total records, got %d", payload.TotalRecords, receivedPayload.TotalRecords)
	}

	if receivedPayload.UniqueDates != payload.UniqueDates {
		t.Errorf("Expected %d unique dates, got %d", payload.UniqueDates, receivedPayload.UniqueDates)
	}

	if receivedPayload.JobStatus != payload.JobStatus {
		t.Errorf("Expected job status %s, got %s", payload.JobStatus, receivedPayload.JobStatus)
	}
}

func TestSendWebhook_EmptyURL(t *testing.T) {
	// Should not error when webhook URL is empty
	payload := &WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		JobStatus: "success",
	}

	ctx := context.Background()
	err := SendWebhook(ctx, "", payload)
	if err != nil {
		t.Errorf("Expected no error with empty URL, got: %v", err)
	}
}

func TestSendWebhook_ServerError(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	payload := &WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		JobStatus: "success",
	}

	ctx := context.Background()
	err := SendWebhook(ctx, server.URL, payload)
	if err == nil {
		t.Error("Expected error when server returns 500, got nil")
	}
}

func TestSendWebhook_Timeout(t *testing.T) {
	// Create test server that hangs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(15 * time.Second) // Longer than webhook timeout
	}))
	defer server.Close()

	payload := &WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		JobStatus: "success",
	}

	ctx := context.Background()
	err := SendWebhook(ctx, server.URL, payload)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
}

func TestWebhookPayload_JSONSerialization(t *testing.T) {
	payload := &WebhookPayload{
		Timestamp:        "2025-10-12T10:00:00Z",
		TotalRecords:     100,
		UniqueDates:      30,
		UniqueModels:     5,
		UniqueProviders:  3,
		DateRangeStart:   "2025-09-01",
		DateRangeEnd:     "2025-09-30",
		TotalRequests:    1000,
		TotalUsage:       50.25,
		RecordsImported:  10,
		JobDuration:      "5s",
		JobStatus:        "success",
	}

	// Serialize to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Deserialize back
	var decoded WebhookPayload
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	// Verify fields
	if decoded.TotalRecords != payload.TotalRecords {
		t.Errorf("Expected %d total records, got %d", payload.TotalRecords, decoded.TotalRecords)
	}

	if decoded.JobStatus != payload.JobStatus {
		t.Errorf("Expected job status %s, got %s", payload.JobStatus, decoded.JobStatus)
	}
}

func TestWebhookPayload_ErrorFields(t *testing.T) {
	payload := &WebhookPayload{
		Timestamp:    "2025-10-12T10:00:00Z",
		JobStatus:    "error",
		ErrorMessage: "failed to connect to API",
	}

	// Serialize to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	// Verify error message is included
	var decoded map[string]interface{}
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded["error_message"] != "failed to connect to API" {
		t.Errorf("Expected error_message in JSON, got: %v", decoded)
	}
}
