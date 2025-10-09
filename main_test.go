package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestPrintSummary(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	summary := &Summary{
		TotalRecords:          100,
		UniqueDates:           30,
		UniqueModels:          5,
		UniqueProviders:       3,
		TotalRequests:         1000,
		TotalUsage:            45.67,
		TotalPromptTokens:     50000,
		TotalCompletionTokens: 25000,
		TotalReasoningTokens:  5000,
	}

	PrintSummary(summary, "test.db")

	// Restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected information
	expectedStrings := []string{
		"Database Summary",
		"Total records in database: 100",
		"Date range: 30 unique dates",
		"Models used: 5 unique models",
		"Providers: 3 unique providers",
		"Total API requests: 1000",
		"Total usage cost: $45.6700",
		"Prompt: 50000",
		"Completion: 25000",
		"Reasoning: 5000",
		"Database saved to: test.db",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Expected output to contain %q, but it didn't", expected)
		}
	}
}

func TestPrintSummary_NoReasoningTokens(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	summary := &Summary{
		TotalRecords:          50,
		UniqueDates:           15,
		UniqueModels:          3,
		UniqueProviders:       2,
		TotalRequests:         500,
		TotalUsage:            20.0,
		TotalPromptTokens:     25000,
		TotalCompletionTokens: 12500,
		TotalReasoningTokens:  0, // No reasoning tokens
	}

	PrintSummary(summary, "test.db")

	// Restore stdout
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify reasoning tokens line is not present when zero
	if strings.Contains(output, "Reasoning:") {
		t.Error("Expected output to NOT contain 'Reasoning:' when reasoning tokens are 0")
	}

	// Verify other expected information is present
	if !strings.Contains(output, "Total records in database: 50") {
		t.Error("Expected output to contain record count")
	}
	if !strings.Contains(output, "Total usage cost: $20.0000") {
		t.Error("Expected output to contain usage cost")
	}
}
