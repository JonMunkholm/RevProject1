-- name: CreateProduct :one
INSERT INTO products (
    Company_ID,
    Prod_Name,
    Rev_Assessment,
    Over_Time_Percent,
    Point_In_Time_Percent,
    Standalone_Selling_Price_Method,
    Standalone_Selling_Price_Price_High,
    Standalone_Selling_Price_Price_Low,
    Default_Currency
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
)
RETURNING *;

-- name: DeleteProduct :exec
DELETE FROM products
WHERE ID = $1
AND Company_ID = $2;

-- name: SetProductActiveStatus :exec
UPDATE products
SET Is_Active = $1
WHERE ID = $2
AND Company_ID = $3;

-- name: UpdateProduct :one
UPDATE products
SET
    Prod_Name = $1,
    Rev_Assessment = $2,
    Over_Time_Percent = $3,
    Point_In_Time_Percent = $4,
    Standalone_Selling_Price_Method = $5,
    Standalone_Selling_Price_Price_High = $6,
    Standalone_Selling_Price_Price_Low = $7,
    Default_Currency = $8,
    Is_Active = $9
WHERE ID = $10
AND Company_ID = $11
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products
WHERE ID = $1
AND Company_ID = $2;

-- name: GetProductByName :one
SELECT * FROM products
WHERE Company_ID = $1
AND Prod_Name = $2;

-- name: GetAllProductsCompany :many
SELECT * FROM products
WHERE Company_ID = $1;

-- name: GetActiveProductsCompany :many
SELECT * FROM products
WHERE Company_ID = $1
AND Is_Active = TRUE;

-- name: DeleteAllProductsCompany :exec
DELETE FROM products
WHERE Company_ID = $1;




-- name: GetAllProducts :many
SELECT * FROM products;


-- name: ResetProducts :exec
Delete FROM products;
