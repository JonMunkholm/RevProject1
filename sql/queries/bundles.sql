-- name: CreateBundle :one
INSERT INTO bundles (Bundle_Name, Company_ID)
VALUES (
    $1,
    $2
)
RETURNING *;

-- name: DeleteBundle :exec
DELETE FROM bundles
WHERE ID = $1
AND Company_ID = $2;

-- name: SetBundleActiveStatus :exec
UPDATE bundles
SET Is_Active = $1
WHERE ID = $2
AND Company_ID = $3;

-- name: UpdateBundle :one
UPDATE bundles
SET
    Bundle_Name = $1,
    Is_Active = $2
WHERE ID = $3
AND Company_ID = $4
RETURNING *;

-- name: GetBundle :one
SELECT * FROM bundles
WHERE ID = $1
AND Company_ID = $2;

-- name: GetBundleByName :one
SELECT * FROM bundles
WHERE Company_ID = $1
AND Bundle_Name = $2;

-- name: GetAllBundlesCompany :many
SELECT * FROM bundles
Where Company_ID = $1;

-- name: GetActiveBundlesCompany :many
SELECT * FROM bundles
WHERE Company_ID = $1
AND Is_Active = TRUE;



-- name: GetAllBundles :many
SELECT * FROM bundles;

-- name: ResetBundles :exec
DELETE FROM bundles;




-- name: AddProductToBundle :one
INSERT INTO bundle_products (Bundle_ID, Product_ID, Company_ID)
VALUES (
    $1,
    $2,
    $3
)
RETURNING *;

-- name: DeleteProductFromBundle :exec
DELETE FROM bundle_products
WHERE Bundle_ID = $1
AND Product_ID = $2
AND Company_ID = $3;

-- name: GetBundleProducts :many
SELECT * FROM bundle_products
WHERE Bundle_ID = $1
AND Company_ID = $2;

-- name: ClearBundleProducts :exec
DELETE FROM bundle_products
WHERE Bundle_ID = $1
AND Company_ID = $2;

-- name: ResetBundleProducts :exec
DELETE FROM bundle_products;




-- name: GetBundleProductDetails :many
SELECT
    bp.Bundle_ID,
    bp.Company_ID,
    p.*
FROM bundle_products bp
INNER JOIN products p ON p.ID = bp.Product_ID
WHERE bp.Bundle_ID = $1
AND bp.Company_ID = $2;


-- name: GetBundlesForProduct :many
SELECT b.*
FROM bundles b
INNER JOIN bundle_products bp ON bp.Bundle_ID = b.ID
WHERE bp.Product_ID = $1
AND bp.Company_ID = $2;
