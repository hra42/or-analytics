-- Breakdown by model and provider
SELECT
    model,
    provider_name,
    COUNT(*) as record_count,
    SUM(requests) as total_requests,
    SUM(usage) as total_spend,
    SUM(prompt_tokens) as total_prompt_tokens,
    SUM(completion_tokens) as total_completion_tokens,
    SUM(reasoning_tokens) as total_reasoning_tokens
FROM activity
GROUP BY model, provider_name
ORDER BY total_spend DESC
LIMIT 20;
