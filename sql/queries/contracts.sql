-- name: CreateContract :one
INSERT INTO contracts (
    Company_ID,
    Customer_ID,
    Start_Date,
    End_Date,
    Is_Final,
    Contract_URL
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: DeleteContract :exec
DELETE FROM contracts
WHERE ID = $1
AND Company_ID = $2;

-- name: UpdateContract :one
UPDATE contracts
SET
    Customer_ID = $1,
    Start_Date = $2,
    End_Date = $3,
    Is_Final = $4,
    Contract_URL = $5
WHERE ID = $6
AND Company_ID = $7
RETURNING *;

-- name: GetContract :one
SELECT * FROM contracts
WHERE ID = $1
AND Company_ID = $2;

-- name: GetContractsByCustomer :many
SELECT * FROM contracts
WHERE Company_ID = $1
AND Customer_ID = $2;

-- name: GetAllContractsCompany :many
SELECT * FROM contracts
WHERE Company_ID = $1;

-- name: GetFinalContractsCompany :many
SELECT * FROM contracts
WHERE Company_ID = $1
AND Is_Final = TRUE;



-- name: GetAllContracts :many
SELECT * FROM contracts;

-- name: ResetContracts :exec
Delete FROM contracts;
