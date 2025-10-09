package main

import (
	"testing"

	"github.com/hra42/openrouter-go"
)

func TestNormalizeDate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard date",
			input:    "2025-10-09",
			expected: "2025-10-09",
		},
		{
			name:     "date with timestamp",
			input:    "2025-10-09T00:00:00Z",
			expected: "2025-10-09",
		},
		{
			name:     "date with full timestamp",
			input:    "2025-10-09T12:34:56.789Z",
			expected: "2025-10-09",
		},
		{
			name:     "short date",
			input:    "2025-10-9",
			expected: "2025-10-9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeDate(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeDate(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertActivityData_Empty(t *testing.T) {
	data := []openrouter.ActivityData{}
	result := ConvertActivityData(data)

	if len(result) != 0 {
		t.Errorf("Expected empty slice, got %d records", len(result))
	}
}

func TestConvertActivityData_Single(t *testing.T) {
	data := []openrouter.ActivityData{
		{
			Date:               "2025-10-09",
			Model:              "test/model",
			ProviderName:       "test-provider",
			Requests:           10.0,
			Usage:              0.5,
			PromptTokens:       1000.0,
			CompletionTokens:   500.0,
			ReasoningTokens:    100.0,
			BYOKUsageInference: 0.05,
		},
	}

	result := ConvertActivityData(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(result))
	}

	r := result[0]
	if r.Date != "2025-10-09" {
		t.Errorf("Expected date '2025-10-09', got '%s'", r.Date)
	}
	if r.Model != "test/model" {
		t.Errorf("Expected model 'test/model', got '%s'", r.Model)
	}
	if r.ProviderName != "test-provider" {
		t.Errorf("Expected provider 'test-provider', got '%s'", r.ProviderName)
	}
	if r.Requests != 10.0 {
		t.Errorf("Expected requests 10.0, got %f", r.Requests)
	}
	if r.Usage != 0.5 {
		t.Errorf("Expected usage 0.5, got %f", r.Usage)
	}
	if r.PromptTokens != 1000.0 {
		t.Errorf("Expected prompt tokens 1000.0, got %f", r.PromptTokens)
	}
	if r.CompletionTokens != 500.0 {
		t.Errorf("Expected completion tokens 500.0, got %f", r.CompletionTokens)
	}
	if r.ReasoningTokens != 100.0 {
		t.Errorf("Expected reasoning tokens 100.0, got %f", r.ReasoningTokens)
	}
	if r.BYOKUsageInference != 0.05 {
		t.Errorf("Expected BYOK usage 0.05, got %f", r.BYOKUsageInference)
	}
}

func TestConvertActivityData_Multiple(t *testing.T) {
	data := []openrouter.ActivityData{
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
			Date:             "2025-10-10",
			Model:            "model2",
			ProviderName:     "provider2",
			Requests:         5.0,
			Usage:            0.3,
			PromptTokens:     500.0,
			CompletionTokens: 250.0,
		},
		{
			Date:             "2025-10-11",
			Model:            "model3",
			ProviderName:     "provider3",
			Requests:         8.0,
			Usage:            0.4,
			PromptTokens:     800.0,
			CompletionTokens: 400.0,
		},
	}

	result := ConvertActivityData(data)

	if len(result) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(result))
	}

	// Verify first record
	if result[0].Model != "model1" {
		t.Errorf("Expected first model 'model1', got '%s'", result[0].Model)
	}

	// Verify second record
	if result[1].Model != "model2" {
		t.Errorf("Expected second model 'model2', got '%s'", result[1].Model)
	}

	// Verify third record
	if result[2].Model != "model3" {
		t.Errorf("Expected third model 'model3', got '%s'", result[2].Model)
	}
}

func TestConvertActivityData_DateNormalization(t *testing.T) {
	data := []openrouter.ActivityData{
		{
			Date:         "2025-10-09T12:34:56Z",
			Model:        "test/model",
			ProviderName: "test-provider",
			Requests:     10.0,
		},
	}

	result := ConvertActivityData(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(result))
	}

	if result[0].Date != "2025-10-09" {
		t.Errorf("Expected normalized date '2025-10-09', got '%s'", result[0].Date)
	}
}

func TestConvertActivityData_ZeroValues(t *testing.T) {
	data := []openrouter.ActivityData{
		{
			Date:               "2025-10-09",
			Model:              "test/model",
			ProviderName:       "test-provider",
			Requests:           5.0,
			Usage:              0.25,
			PromptTokens:       100.0,
			CompletionTokens:   50.0,
			ReasoningTokens:    0.0, // Zero reasoning tokens
			BYOKUsageInference: 0.0, // Zero BYOK usage
		},
	}

	result := ConvertActivityData(data)

	if len(result) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(result))
	}

	if result[0].ReasoningTokens != 0.0 {
		t.Errorf("Expected reasoning tokens 0.0, got %f", result[0].ReasoningTokens)
	}
	if result[0].BYOKUsageInference != 0.0 {
		t.Errorf("Expected BYOK usage 0.0, got %f", result[0].BYOKUsageInference)
	}
}
