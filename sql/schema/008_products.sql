-- +goose Up
ALTER TABLE products
  ALTER COLUMN Standalone_Selling_Price_Price_High TYPE NUMERIC,
  ALTER COLUMN Standalone_Selling_Price_Price_Low  TYPE NUMERIC;

-- +goose Down
ALTER TABLE products
  ALTER COLUMN Standalone_Selling_Price_Price_High TYPE NUMERIC(10,9)
    USING CASE
            WHEN Standalone_Selling_Price_Price_High BETWEEN -9.999999999 AND 9.999999999
                THEN Standalone_Selling_Price_Price_High
            WHEN Standalone_Selling_Price_Price_High < -9.999999999
                THEN -9.999999999
            ELSE 9.999999999
         END,
  ALTER COLUMN Standalone_Selling_Price_Price_Low  TYPE NUMERIC(10,9)
    USING CASE
            WHEN Standalone_Selling_Price_Price_Low BETWEEN -9.999999999 AND 9.999999999
                THEN Standalone_Selling_Price_Price_Low
            WHEN Standalone_Selling_Price_Price_Low < -9.999999999
                THEN -9.999999999
            ELSE 9.999999999
         END;
