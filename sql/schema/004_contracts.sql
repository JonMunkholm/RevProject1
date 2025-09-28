-- +goose Up
CREATE TABLE contracts (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Company_ID UUID NOT NULL,
    Customer_ID UUID NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Start_Date TIMESTAMP NOT NULL,
    End_Date TIMESTAMP NOT NULL,
    Is_Final BOOLEAN NOT NULL DEFAULT FALSE,
    Contract_URL TEXT,
    CONSTRAINT fk_contracts_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE,
    CONSTRAINT fk_contracts_customer_id
        FOREIGN KEY (Customer_ID)
        REFERENCES customers(ID),
    CONSTRAINT CHK_End_Date CHECK (End_Date > Start_Date)
);

CREATE TRIGGER update_contracts_updated_at BEFORE UPDATE ON contracts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_contracts_company_id ON contracts (Company_ID);
CREATE INDEX idx_contracts_company_customer ON contracts (Company_ID, Customer_ID);
CREATE INDEX idx_contracts_company_is_final ON contracts (Company_ID) WHERE Is_Final;

-- +goose Down
DROP INDEX IF EXISTS idx_contracts_company_is_final;
DROP INDEX IF EXISTS idx_contracts_company_customer;
DROP INDEX IF EXISTS idx_contracts_company_id;
DROP TABLE contracts;
