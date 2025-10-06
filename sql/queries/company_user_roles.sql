-- name: UpsertCompanyUserRole :one
INSERT INTO company_user_roles (company_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (company_id, user_id)
    DO UPDATE SET role = EXCLUDED.role
RETURNING *;

-- name: ListCompanyRolesForUser :many
SELECT company_id, user_id, role, created_at, updated_at
FROM company_user_roles
WHERE user_id = $1
ORDER BY company_id;

-- name: ListCompanyUserRoles :many
SELECT company_id, user_id, role, created_at, updated_at
FROM company_user_roles
WHERE company_id = $1
ORDER BY user_id;

-- name: DeleteCompanyUserRole :exec
DELETE FROM company_user_roles
WHERE company_id = $1
  AND user_id = $2;
