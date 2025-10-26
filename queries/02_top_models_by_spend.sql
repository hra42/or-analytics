-- Top 10 models by total spend
SELECT
    model,
    COUNT(*) as record_count,
    SUM(requests) as total_requests,
    SUM(usage) as total_spend,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(reasoning_tokens) as total_reasoning_tokens,
    SUM(usage) / NULLIF(SUM(requests), 0) as avg_cost_per_request,
    SUM(prompt_tokens + completion_tokens + reasoning_tokens) / NULLIF(SUM(requests), 0) as avg_tokens_per_request
FROM analytics
GROUP BY model
ORDER BY total_spend DESC
LIMIT 10;
