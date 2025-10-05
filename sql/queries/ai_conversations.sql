-- name: CreateAIConversationSession :one
INSERT INTO ai_conversation_sessions (
    company_id,
    user_id,
    provider_id,
    title,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    $4,
    COALESCE($5, '{}'::jsonb)
)
RETURNING *;

-- name: GetAIConversationSession :one
SELECT *
FROM ai_conversation_sessions
WHERE id = $1
  AND company_id = $2;

-- name: UpdateAIConversationSessionTitle :exec
UPDATE ai_conversation_sessions
SET title = $3,
    updated_at = now()
WHERE id = $1
  AND company_id = $2;

-- name: ListAIConversationSessionsByCompany :many
SELECT *
FROM ai_conversation_sessions
WHERE company_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteAIConversationSession :exec
DELETE FROM ai_conversation_sessions
WHERE id = $1
  AND company_id = $2;

-- name: InsertAIConversationMessage :one
INSERT INTO ai_conversation_messages (
    session_id,
    role,
    content,
    metadata
) VALUES (
    $1,
    $2,
    $3,
    COALESCE($4, '{}'::jsonb)
)
RETURNING *;

-- name: ListAIConversationMessages :many
SELECT *
FROM ai_conversation_messages
WHERE session_id = $1
ORDER BY created_at ASC;

-- name: DeleteAIConversationMessagesForSession :exec
DELETE FROM ai_conversation_messages
WHERE session_id = $1;
