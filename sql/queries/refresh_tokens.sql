-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (
    User_ID,
    Token_Hash,
    Issued_IP,
    User_Agent,
    Expires_At
)
VALUES (
    sqlc.arg(user_id),
    sqlc.arg(token_hash),
    sqlc.arg(issued_ip),
    sqlc.arg(user_agent),
    sqlc.arg(expires_at)
);

-- name: GetRefreshTokenByHash :one
SELECT
    ID,
    User_ID,
    Token_Hash,
    Issued_IP,
    User_Agent,
    Created_At,
    Updated_At,
    Expires_At,
    Revoked_At
FROM refresh_tokens
WHERE Token_Hash = sqlc.arg(token_hash)
  AND (Revoked_At IS NULL OR sqlc.arg(include_revoked));

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET Revoked_At = COALESCE(sqlc.arg(revoked_at), CURRENT_TIMESTAMP)
WHERE ID = sqlc.arg(id)
  AND Revoked_At IS NULL;
