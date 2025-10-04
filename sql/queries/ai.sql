-- name: InsertAIProviderCredential :one
INSERT INTO ai_provider_credentials (
    company_id,
    user_id,
    provider_id,
    credential_cipher,
    credential_hash,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    COALESCE($6, '{}'::jsonb)
)
RETURNING *;

-- name: UpdateAIProviderCredential :one
UPDATE ai_provider_credentials
SET
    credential_cipher = $3,
    credential_hash   = $4,
    metadata          = COALESCE($5, metadata),
    updated_at        = now(),
    rotated_at        = now()
WHERE company_id = $1
  AND user_id    = $2
  AND provider_id = $6
RETURNING *;

-- name: GetAIProviderCredential :one
SELECT *
FROM ai_provider_credentials
WHERE company_id = $1
  AND user_id    = $2
  AND provider_id = $3;

-- name: TouchAIProviderCredential :exec
UPDATE ai_provider_credentials
SET last_used_at = now(),
    updated_at   = now()
WHERE company_id = $1
  AND user_id    = $2
  AND provider_id = $3;

-- name: DeleteAIProviderCredential :exec
DELETE FROM ai_provider_credentials
WHERE company_id = $1
  AND user_id    = $2
  AND provider_id = $3;

-- name: UpsertAIUserPreference :one
INSERT INTO ai_user_preferences (company_id, user_id, provider_id, model, metadata)
VALUES ($1, $2, $3, $4, COALESCE($5, '{}'::jsonb))
ON CONFLICT (company_id, user_id) DO UPDATE
SET provider_id = EXCLUDED.provider_id,
    model       = EXCLUDED.model,
    metadata    = EXCLUDED.metadata,
    updated_at  = now()
RETURNING *;

-- name: GetAIUserPreference :one
SELECT *
FROM ai_user_preferences
WHERE company_id = $1
  AND user_id    = $2;

-- name: DeleteAIUserPreference :exec
DELETE FROM ai_user_preferences
WHERE company_id = $1
  AND user_id    = $2;

-- name: InsertAIToolInvocation :exec
INSERT INTO ai_tool_invocations (
    user_id,
    provider_id,
    tool_name,
    status,
    request,
    response,
    error_message
) VALUES (
    $1,
    $2,
    $3,
    COALESCE($4, 'success'),
    $5,
    $6,
    $7
);

-- name: ListAIToolInvocationsByUser :many
SELECT *
FROM ai_tool_invocations
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAIToolInvocationsByProvider :many
SELECT *
FROM ai_tool_invocations
WHERE provider_id = $1
  AND created_at BETWEEN $2 AND $3
ORDER BY created_at DESC;
