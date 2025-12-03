package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookPayload contains metrics to send to the webhook
type WebhookPayload struct {
	Timestamp        string  `json:"timestamp"`
	TotalRecords     int     `json:"total_records"`
	UniqueDates      int     `json:"unique_dates"`
	UniqueModels     int     `json:"unique_models"`
	UniqueProviders  int     `json:"unique_providers"`
	DateRangeStart   string  `json:"date_range_start,omitempty"`
	DateRangeEnd     string  `json:"date_range_end,omitempty"`
	TotalRequests    float64 `json:"total_requests"`
	TotalUsage       float64 `json:"total_usage"`
	RecordsImported  int     `json:"records_imported"`
	JobDuration      string  `json:"job_duration"`
	JobStatus        string  `json:"job_status"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

// GetDatabaseMetrics retrieves comprehensive metrics from the database
func GetDatabaseMetrics(db *sql.DB) (*WebhookPayload, error) {
	payload := &WebhookPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		JobStatus: "success",
	}

	// Get total record count
	err := db.QueryRow("SELECT COUNT(*) FROM analytics").Scan(&payload.TotalRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to get total records: %w", err)
	}

	// Get unique counts
	err = db.QueryRow("SELECT COUNT(DISTINCT date) FROM analytics").Scan(&payload.UniqueDates)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique dates: %w", err)
	}

	err = db.QueryRow("SELECT COUNT(DISTINCT model) FROM analytics").Scan(&payload.UniqueModels)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique models: %w", err)
	}

	err = db.QueryRow("SELECT COUNT(DISTINCT provider_name) FROM analytics").Scan(&payload.UniqueProviders)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique providers: %w", err)
	}

	// Get date range
	var startDate, endDate sql.NullString
	err = db.QueryRow("SELECT MIN(date)::VARCHAR, MAX(date)::VARCHAR FROM analytics").Scan(&startDate, &endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get date range: %w", err)
	}

	if startDate.Valid {
		payload.DateRangeStart = startDate.String
	}
	if endDate.Valid {
		payload.DateRangeEnd = endDate.String
	}

	// Get total requests and usage
	var requests, usage sql.NullFloat64
	err = db.QueryRow("SELECT COALESCE(SUM(requests), 0), COALESCE(SUM(byok_usage_inference), 0) FROM analytics").Scan(&requests, &usage)
	if err != nil {
		return nil, fmt.Errorf("failed to get totals: %w", err)
	}

	if requests.Valid {
		payload.TotalRequests = requests.Float64
	}
	if usage.Valid {
		payload.TotalUsage = usage.Float64
	}

	return payload, nil
}

// SendWebhook sends metrics to the configured webhook URL
func SendWebhook(ctx context.Context, webhookURL string, payload *WebhookPayload) error {
	if webhookURL == "" {
		return nil // Webhook not configured, skip silently
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "OR-Analytics/1.0")

	// Send request with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status code: %d", resp.StatusCode)
	}

	return nil
}
