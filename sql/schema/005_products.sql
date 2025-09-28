-- +goose Up
CREATE TABLE products (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Prod_Name CITEXT NOT NULL,
    Rev_Assessment VARCHAR(20) NOT NULL,
    Over_Time_Percent NUMERIC(5,4) NOT NULL,
    Point_In_Time_Percent NUMERIC(5,4) NOT NULL,
    Standalone_Selling_Price_Method VARCHAR(50) NOT NULL,
    Standalone_Selling_Price_Price_High NUMERIC(10,9) NOT NULL,
    Standalone_Selling_Price_Price_Low NUMERIC(10,9) NOT NULL,
    Company_ID UUID NOT NULL,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    Default_Currency VARCHAR(3) NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT UQ_products_company_name UNIQUE (Company_ID, Prod_Name),
    CONSTRAINT UQ_products_company_id UNIQUE (ID, Company_ID),
    CONSTRAINT CHK_prod_name_not_blank CHECK (BTRIM(Prod_Name) <> ''),
    CONSTRAINT fk_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE,
    CONSTRAINT CHK_Rev_Assessment CHECK (Rev_Assessment IN ('over_time', 'point_in_time', 'split')),
    CONSTRAINT CHK_Over_Time_Range CHECK (Over_Time_Percent >= 0.0000 AND Over_Time_Percent <= 1.0000),
    CONSTRAINT CHK_Point_In_Time_Range CHECK (Point_In_Time_Percent >= 0.0000 AND Point_In_Time_Percent <= 1.0000),
    CONSTRAINT CHK_SSP CHECK (Standalone_Selling_Price_Method IN ('observable', 'adjusted_market', 'cost_plus', 'residual')),
    CONSTRAINT CHK_SSP_PRICE_ORDER CHECK (
        Standalone_Selling_Price_Price_Low >= 0
        AND Standalone_Selling_Price_Price_High >= Standalone_Selling_Price_Price_Low
    ),
    CONSTRAINT CHK_DEFAULT_CURRENCY_FORMAT CHECK (Default_Currency ~ '^[A-Z]{3}$'),
    CONSTRAINT CHK_REV_ASSESSMENT_PERCENTS CHECK (
        (Rev_Assessment = 'over_time' AND Over_Time_Percent = 1 AND Point_In_Time_Percent = 0)
        OR (Rev_Assessment = 'point_in_time' AND Over_Time_Percent = 0 AND Point_In_Time_Percent = 1)
        OR (Rev_Assessment = 'split' AND Over_Time_Percent + Point_In_Time_Percent = 1)
    )
);

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_products_company_id ON products (Company_ID);
CREATE INDEX idx_products_company_active ON products (Company_ID) WHERE Is_Active;

-- +goose Down
DROP INDEX IF EXISTS idx_products_company_active;
DROP INDEX IF EXISTS idx_products_company_id;
DROP TRIGGER IF EXISTS update_products_updated_at ON products;
DROP TABLE products;
