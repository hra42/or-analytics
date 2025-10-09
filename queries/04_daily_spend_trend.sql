-- Daily spending trend
SELECT
    date,
    COUNT(DISTINCT model) as models_used,
    SUM(requests) as total_requests,
    SUM(usage) as daily_spend,
    SUM(prompt_tokens + completion_tokens + reasoning_tokens) as total_tokens
FROM activity
GROUP BY date
ORDER BY date DESC;
