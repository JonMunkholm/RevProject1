-- +goose Up
CREATE TABLE customers (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Customer_Name CITEXT NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    Company_ID UUID NOT NULL,
    CONSTRAINT UQ_customers_company_name UNIQUE (Company_ID, Customer_Name),
    CONSTRAINT CHK_customer_name_not_blank CHECK (BTRIM(Customer_Name) <> ''),
    CONSTRAINT fk_customers_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE
);

CREATE TRIGGER update_customers_updated_at BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_customers_company_id ON customers (Company_ID);
CREATE INDEX idx_customers_company_active ON customers (Company_ID) WHERE Is_Active;

-- +goose Down
DROP INDEX IF EXISTS idx_customers_company_active;
DROP INDEX IF EXISTS idx_customers_company_id;
DROP TABLE customers;
