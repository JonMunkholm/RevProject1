-- name: InsertAIDocumentJob :one
INSERT INTO ai_document_jobs (
    company_id,
    user_id,
    provider_id,
    status,
    request
) VALUES (
    $1,
    $2,
    $3,
    COALESCE($4, 'queued'),
    $5
)
RETURNING *;

-- name: UpdateAIDocumentJobStatus :exec
UPDATE ai_document_jobs
SET status        = $3,
    error_message = $4,
    updated_at    = now(),
    completed_at  = CASE WHEN $3 IN ('completed', 'failed') THEN now() ELSE completed_at END
WHERE id = $1
  AND company_id = $2;

-- name: UpdateAIDocumentJobResponse :exec
UPDATE ai_document_jobs
SET response     = $3,
    status       = 'completed',
    updated_at   = now(),
    completed_at = now()
WHERE id = $1
  AND company_id = $2;

-- name: GetAIDocumentJob :one
SELECT *
FROM ai_document_jobs
WHERE id = $1
  AND company_id = $2;

-- name: GetNextQueuedAIDocumentJob :one
SELECT *
FROM ai_document_jobs
WHERE status = 'queued'
ORDER BY created_at ASC
LIMIT 1;

-- name: ListAIDocumentJobsByCompany :many
SELECT *
FROM ai_document_jobs
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteAIDocumentJob :exec
DELETE FROM ai_document_jobs
WHERE id = $1
  AND company_id = $2;
