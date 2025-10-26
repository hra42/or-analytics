-- Recent activity (last 7 days)
SELECT
    date,
    model,
    provider_name,
    requests,
    usage as spend,
    prompt_tokens,
    completion_tokens,
    reasoning_tokens
FROM analytics
WHERE date >= CURRENT_DATE - INTERVAL '7 days'
ORDER BY date DESC, usage DESC;
