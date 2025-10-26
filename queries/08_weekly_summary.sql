-- Weekly spending summary
SELECT
    STRFTIME(date, '%Y-W%W') as week,
    MIN(date) as week_start,
    MAX(date) as week_end,
    COUNT(DISTINCT model) as unique_models,
    SUM(requests) as total_requests,
    SUM(usage) as weekly_spend,
    SUM(prompt_tokens + completion_tokens + reasoning_tokens) as total_tokens
FROM analytics
GROUP BY STRFTIME(date, '%Y-W%W')
ORDER BY week DESC;
