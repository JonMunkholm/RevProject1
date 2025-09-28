-- name: CreateCompany :one
INSERT INTO companies (Company_Name)
VALUES ($1)
RETURNING *;

-- name: DeleteCompany :exec
DELETE FROM companies
WHERE ID = $1;

-- name: SetCompanyActiveStatus :exec
UPDATE companies
SET Is_Active = $1
WHERE ID = $2;

-- name: UpdateCompany :one
UPDATE companies
SET
    Company_Name = $1,
    Is_Active = $2
WHERE ID = $3
RETURNING *;

-- name: GetCompany :one
SELECT * FROM companies
WHERE ID = $1;

-- name: GetCompanyByName :one
SELECT * FROM companies
WHERE Company_Name = $1;

-- name: GetActiveCompanies :many
SELECT * FROM companies
WHERE Is_Active = TRUE;



-- name: GetAllCompanies :many
SELECT * FROM companies;

-- name: ResetCompanies :exec
DELETE FROM companies;
