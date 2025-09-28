-- name: CreateCustomer :one
INSERT INTO customers (Customer_Name, Company_ID)
VALUES (
    $1,
    $2
)
RETURNING *;

-- name: DeleteCustomer :exec
DELETE FROM customers
WHERE ID = $1
AND Company_ID = $2;

-- name: SetCustomerActiveStatus :exec
UPDATE customers
SET Is_Active = $1
WHERE ID = $2
AND Company_ID = $3;

-- name: UpdateCustomer :one
UPDATE customers
SET
    Customer_Name = $1,
    Is_Active = $2
WHERE ID = $3
AND Company_ID = $4
RETURNING *;

-- name: GetCustomer :one
SELECT * FROM customers
WHERE ID = $1
AND Company_ID = $2;

-- name: GetCustomerByName :one
SELECT * FROM customers
WHERE Company_ID = $1
AND Customer_Name = $2;

-- name: GetAllCustomersCompany :many
SELECT * FROM customers
WHERE Company_ID = $1;

-- name: GetActiveCustomersCompany :many
SELECT * FROM customers
WHERE Company_ID = $1
AND Is_Active = TRUE;



-- name: GetAllCustomers :many
SELECT * FROM customers;

-- name: ResetCustomers :exec
Delete FROM customers;
