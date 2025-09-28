-- name: CreatePerformanceObligation :one
INSERT INTO performance_obligations (
    Performance_Obligations_Name,
    Contract_ID,
    Start_Date,
    End_Date,
    Functional_Currency,
    Discount,
    Transaction_Price
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: DeletePerformanceObligation :exec
DELETE FROM performance_obligations po
USING contracts c
WHERE po.ID = $1
AND c.ID = po.Contract_ID
AND c.Company_ID = $2;

-- name: UpdatePerformanceObligation :one
UPDATE performance_obligations po
SET
    Performance_Obligations_Name = $1,
    Contract_ID = $2,
    Start_Date = $3,
    End_Date = $4,
    Functional_Currency = $5,
    Discount = $6,
    Transaction_Price = $7
FROM contracts current_contract,
     contracts new_contract
WHERE po.ID = $8
AND current_contract.ID = po.Contract_ID
AND current_contract.Company_ID = $9
AND new_contract.ID = $2
AND new_contract.Company_ID = $9
RETURNING po.*;

-- name: GetPerformanceObligation :one
SELECT po.*
FROM performance_obligations po
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE po.ID = $1
AND c.Company_ID = $2;

-- name: GetPerformanceObligationsForContract :many
SELECT po.*
FROM performance_obligations po
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE po.Contract_ID = $1
AND c.Company_ID = $2
ORDER BY po.Start_Date;

-- name: GetPerformanceObligationsForCompany :many
SELECT po.*
FROM performance_obligations po
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE c.Company_ID = $1
ORDER BY po.Start_Date, po.Performance_Obligations_Name;

-- name: GetAllPerformanceObligations :many
SELECT * FROM performance_obligations;

-- name: ResetPerformanceObligations :exec
DELETE FROM performance_obligations;



-- name: AddProductToPerformanceObligation :one
INSERT INTO product_performance_obligations (Product_ID, Performance_Obligations_ID)
SELECT p.ID, po.ID
FROM performance_obligations po
INNER JOIN contracts c ON c.ID = po.Contract_ID
INNER JOIN products p ON p.ID = $1 AND p.Company_ID = $3
WHERE po.ID = $2
AND c.Company_ID = $3
RETURNING *;

-- name: DeleteProductFromPerformanceObligation :exec
DELETE FROM product_performance_obligations ppo
USING performance_obligations po,
      contracts c,
      products p
WHERE ppo.Product_ID = $1
AND ppo.Performance_Obligations_ID = $2
AND po.ID = ppo.Performance_Obligations_ID
AND c.ID = po.Contract_ID
AND p.ID = ppo.Product_ID
AND c.Company_ID = $3
AND p.Company_ID = $3;

-- name: GetPerformanceObligationProducts :many
SELECT p.*
FROM product_performance_obligations ppo
INNER JOIN products p ON p.ID = ppo.Product_ID
INNER JOIN performance_obligations po ON po.ID = ppo.Performance_Obligations_ID
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE ppo.Performance_Obligations_ID = $1
AND c.Company_ID = $2
ORDER BY p.Prod_Name;

-- name: GetPerformanceObligationsForProduct :many
SELECT po.*
FROM product_performance_obligations ppo
INNER JOIN performance_obligations po ON po.ID = ppo.Performance_Obligations_ID
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE ppo.Product_ID = $1
AND c.Company_ID = $2
ORDER BY po.Start_Date;

-- name: ClearPerformanceObligationProducts :exec
DELETE FROM product_performance_obligations ppo
USING performance_obligations po,
      contracts c
WHERE ppo.Performance_Obligations_ID = $1
AND po.ID = ppo.Performance_Obligations_ID
AND c.ID = po.Contract_ID
AND c.Company_ID = $2;

-- name: ResetProductPerformanceObligations :exec
DELETE FROM product_performance_obligations;



-- name: AddBundleToPerformanceObligation :one
INSERT INTO bundle_performance_obligations (Bundle_ID, Performance_Obligations_ID)
SELECT b.ID, po.ID
FROM performance_obligations po
INNER JOIN contracts c ON c.ID = po.Contract_ID
INNER JOIN bundles b ON b.ID = $1 AND b.Company_ID = $3
WHERE po.ID = $2
AND c.Company_ID = $3
RETURNING *;

-- name: DeleteBundleFromPerformanceObligation :exec
DELETE FROM bundle_performance_obligations bpo
USING performance_obligations po,
      contracts c,
      bundles b
WHERE bpo.Bundle_ID = $1
AND bpo.Performance_Obligations_ID = $2
AND po.ID = bpo.Performance_Obligations_ID
AND c.ID = po.Contract_ID
AND b.ID = bpo.Bundle_ID
AND c.Company_ID = $3
AND b.Company_ID = $3;

-- name: GetPerformanceObligationBundles :many
SELECT b.*
FROM bundle_performance_obligations bpo
INNER JOIN bundles b ON b.ID = bpo.Bundle_ID
INNER JOIN performance_obligations po ON po.ID = bpo.Performance_Obligations_ID
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE bpo.Performance_Obligations_ID = $1
AND c.Company_ID = $2
ORDER BY b.Bundle_Name;

-- name: GetPerformanceObligationsForBundle :many
SELECT po.*
FROM bundle_performance_obligations bpo
INNER JOIN performance_obligations po ON po.ID = bpo.Performance_Obligations_ID
INNER JOIN contracts c ON c.ID = po.Contract_ID
WHERE bpo.Bundle_ID = $1
AND c.Company_ID = $2
ORDER BY po.Start_Date;

-- name: ClearPerformanceObligationBundles :exec
DELETE FROM bundle_performance_obligations bpo
USING performance_obligations po,
      contracts c
WHERE bpo.Performance_Obligations_ID = $1
AND po.ID = bpo.Performance_Obligations_ID
AND c.ID = po.Contract_ID
AND c.Company_ID = $2;

-- name: ResetBundlePerformanceObligations :exec
DELETE FROM bundle_performance_obligations;
