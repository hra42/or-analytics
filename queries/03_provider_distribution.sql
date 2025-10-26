-- Provider distribution and cost breakdown
SELECT
    provider_name,
    COUNT(DISTINCT model) as models_used,
    COUNT(*) as record_count,
    SUM(requests) as total_requests,
    SUM(usage) as total_spend,
    ROUND(100.0 * SUM(usage) / (SELECT SUM(usage) FROM analytics), 2) as percent_of_total_spend,
    SUM(usage) / NULLIF(SUM(requests), 0) as avg_cost_per_request
FROM analytics
GROUP BY provider_name
ORDER BY total_spend DESC;
