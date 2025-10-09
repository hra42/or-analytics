package main

import (
	"github.com/hra42/openrouter-go"
)

// NormalizeDate ensures date strings are in YYYY-MM-DD format
func NormalizeDate(dateStr string) string {
	if len(dateStr) > 10 {
		// Truncate if it has timestamp
		return dateStr[:10]
	}
	return dateStr
}

// ConvertActivityData converts OpenRouter activity data to ActivityRecord format
func ConvertActivityData(data []openrouter.ActivityData) []ActivityRecord {
	records := make([]ActivityRecord, 0, len(data))

	for _, d := range data {
		records = append(records, ActivityRecord{
			Date:               NormalizeDate(d.Date),
			Model:              d.Model,
			ProviderName:       d.ProviderName,
			Requests:           d.Requests,
			Usage:              d.Usage,
			PromptTokens:       d.PromptTokens,
			CompletionTokens:   d.CompletionTokens,
			ReasoningTokens:    d.ReasoningTokens,
			BYOKUsageInference: d.BYOKUsageInference,
		})
	}

	return records
}
