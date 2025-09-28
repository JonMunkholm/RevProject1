-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE companies (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Company_Name CITEXT NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    CONSTRAINT UQ_company_name UNIQUE (Company_Name),
    CONSTRAINT CHK_company_name_not_blank CHECK (BTRIM(Company_Name) <> '')
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.Updated_At = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE PLPGSQL;
-- +goose StatementEnd

CREATE TRIGGER update_companies_updated_at
BEFORE UPDATE ON companies
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_companies_updated_at ON companies;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS companies;
DROP EXTENSION IF EXISTS citext;
