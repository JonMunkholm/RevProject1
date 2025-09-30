-- name: CreateUser :one
INSERT INTO users (Company_ID, Email, Password_Hash)
VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: DeleteUser :exec
DELETE FROM users
WHERE ID = $1
AND Company_ID = $2;

-- name: SetUserActiveStatus :exec
UPDATE users
SET Is_Active = $1
WHERE ID = $2
AND Company_ID = $3;

-- name: UpdateUser :one
UPDATE users
SET
    Email = $1,
    Is_Active = $2
WHERE ID = $3
AND Company_ID = $4
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE ID = $1
AND Company_ID = $2;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE Company_ID = $1
AND Email = $2;

-- name: GetUserByIDGlobal :one
SELECT * FROM users
WHERE ID = $1;

-- name: GetUserByEmailGlobal :one
SELECT * FROM users
WHERE Email = $1;

-- name: GetAllUsersCompany :many
SELECT * FROM users
WHERE Company_ID = $1;

-- name: GetActiveUsersCompany :many
SELECT * FROM users
WHERE Company_ID = $1
AND Is_Active = TRUE;



-- name: GetAllUsers :many
SELECT * FROM users;

-- name: ResetUsers :exec
Delete FROM users;
