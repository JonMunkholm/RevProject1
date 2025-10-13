-- name: ListAIProviderCatalogEntries :many
SELECT
    id,
    label,
    icon_url,
    description,
    documentation_url,
    capabilities,
    models,
    fields,
    enabled,
    created_at,
    updated_at
FROM ai_provider_catalog
WHERE enabled = TRUE
ORDER BY id;
