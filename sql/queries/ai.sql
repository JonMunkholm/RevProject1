-- name: InsertAIProviderCredential :one
INSERT INTO ai_provider_credentials (
    company_id,
    user_id,
    provider_id,
    credential_cipher,
    credential_hash,
    metadata,
    label,
    is_default,
    last_tested_at
)
VALUES (
    sqlc.arg('company_id'),
    sqlc.narg('user_id'),
    sqlc.arg('provider_id'),
    sqlc.arg('credential_cipher'),
    sqlc.arg('credential_hash'),
    COALESCE(sqlc.narg('metadata'), '{}'::jsonb),
    sqlc.narg('label'),
    COALESCE(sqlc.narg('is_default'), false),
    sqlc.narg('last_tested_at')
)
RETURNING *;

-- name: UpdateAIProviderCredential :one
UPDATE ai_provider_credentials
SET
    credential_cipher = COALESCE(sqlc.narg('credential_cipher'), credential_cipher),
    credential_hash   = COALESCE(sqlc.narg('credential_hash'), credential_hash),
    metadata          = COALESCE(sqlc.narg('metadata'), metadata),
    label             = COALESCE(sqlc.narg('label'), label),
    is_default        = COALESCE(sqlc.narg('is_default'), is_default),
    last_tested_at    = COALESCE(sqlc.narg('last_tested_at'), last_tested_at),
    updated_at        = now(),
    rotated_at        = CASE
        WHEN sqlc.narg('credential_cipher') IS NOT NULL
             AND sqlc.narg('credential_cipher') IS DISTINCT FROM credential_cipher THEN now()
        ELSE rotated_at
    END
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: GetAIProviderCredential :one
SELECT *
FROM ai_provider_credentials
WHERE id = sqlc.arg('id');

-- name: TouchAIProviderCredential :exec
UPDATE ai_provider_credentials
SET last_used_at = now(),
    updated_at   = now()
WHERE company_id = sqlc.arg('company_id')
  AND user_id IS NOT DISTINCT FROM sqlc.narg('user_id')
  AND provider_id = sqlc.arg('provider_id');

-- name: TouchAIProviderCredentialByID :exec
UPDATE ai_provider_credentials
SET last_used_at = now(),
    updated_at   = now()
WHERE id = sqlc.arg('id');

-- name: DeleteAIProviderCredential :exec
DELETE FROM ai_provider_credentials
WHERE company_id = sqlc.arg('company_id')
  AND user_id IS NOT DISTINCT FROM sqlc.narg('user_id')
  AND provider_id = sqlc.arg('provider_id');

-- name: DeleteAIProviderCredentialByID :exec
DELETE FROM ai_provider_credentials
WHERE id = sqlc.arg('id');

-- name: ListAIProviderCredentialsByCompany :many
SELECT *
FROM ai_provider_credentials
WHERE company_id = sqlc.arg('company_id')
ORDER BY provider_id, user_id, is_default DESC, updated_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListAIProviderCredentialsByScope :many
SELECT *
FROM ai_provider_credentials
WHERE company_id = sqlc.arg('company_id')
  AND provider_id = sqlc.arg('provider_id')
  AND (
    (sqlc.narg('user_id')::uuid IS NULL AND user_id IS NULL)
    OR (sqlc.narg('user_id')::uuid IS NOT NULL AND user_id IS NOT DISTINCT FROM sqlc.narg('user_id')::uuid)
  )
ORDER BY is_default DESC, updated_at DESC, created_at DESC;

-- name: ListAIProviderCredentialsForResolver :many
SELECT *
FROM ai_provider_credentials
WHERE company_id = sqlc.arg('company_id')
  AND provider_id = sqlc.arg('provider_id')
  AND (
    user_id IS NULL OR user_id = sqlc.arg('user_id')
  )
ORDER BY
  CASE WHEN user_id = sqlc.arg('user_id') THEN 0 ELSE 1 END,
  is_default DESC,
  updated_at DESC,
  created_at DESC;

-- name: ClearDefaultAIProviderCredentials :exec
UPDATE ai_provider_credentials
SET is_default = false,
    updated_at = now()
WHERE company_id = sqlc.arg('company_id')
  AND provider_id = sqlc.arg('provider_id')
  AND (
    (sqlc.narg('user_id')::uuid IS NULL AND user_id IS NULL)
    OR (sqlc.narg('user_id')::uuid IS NOT NULL AND user_id IS NOT DISTINCT FROM sqlc.narg('user_id')::uuid)
  );

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

-- name: InsertAIProviderCredentialEvent :exec
INSERT INTO ai_provider_credential_events (
    company_id,
    user_id,
    actor_user_id,
    provider_id,
    action,
    metadata_snapshot
) VALUES (
    sqlc.arg('company_id'),
    sqlc.narg('user_id'),
    sqlc.narg('actor_user_id'),
    sqlc.arg('provider_id'),
    sqlc.arg('action'),
    COALESCE(sqlc.narg('metadata_snapshot'), '{}'::jsonb)
);

-- name: ListAIProviderCredentialEvents :many
SELECT *
FROM ai_provider_credential_events
WHERE company_id = sqlc.arg('company_id')
  AND provider_id = sqlc.arg('provider_id')
  AND (
    sqlc.narg('action')::text IS NULL
    OR action = sqlc.narg('action')::text
  )
  AND (
    sqlc.narg('scope')::text IS NULL
    OR (sqlc.narg('scope')::text = 'company' AND user_id IS NULL)
    OR (sqlc.narg('scope')::text = 'user' AND user_id IS NOT NULL)
  )
  AND (
    sqlc.narg('actor_user_id')::uuid IS NULL
    OR actor_user_id = sqlc.narg('actor_user_id')::uuid
  )
ORDER BY created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
