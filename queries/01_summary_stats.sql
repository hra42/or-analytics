-- Summary statistics across all activity data
SELECT
    COUNT(*) as total_records,
    COUNT(DISTINCT date) as unique_dates,
    COUNT(DISTINCT model) as unique_models,
    COUNT(DISTINCT provider_name) as unique_providers,
    SUM(requests) as total_requests,
    SUM(usage) as total_usage,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(reasoning_tokens) as total_reasoning_tokens,
    MIN(date) as earliest_date,
    MAX(date) as latest_date
FROM analytics;
