-- Cost efficiency analysis - cost per 1K tokens
SELECT
    model,
    provider_name,
    SUM(requests) as total_requests,
    SUM(usage) as total_spend,
    SUM(prompt_tokens + completion_tokens + reasoning_tokens) as total_tokens,
    (SUM(usage) / NULLIF(SUM(prompt_tokens + completion_tokens + reasoning_tokens), 0)) * 1000 as cost_per_1k_tokens,
    SUM(prompt_tokens) / NULLIF(SUM(prompt_tokens + completion_tokens + reasoning_tokens), 0) * 100 as prompt_token_percentage
FROM activity
GROUP BY model, provider_name
HAVING SUM(usage) > 0
ORDER BY cost_per_1k_tokens DESC
LIMIT 20;
