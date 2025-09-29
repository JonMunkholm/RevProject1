-- +goose Up
CREATE TABLE users (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Company_ID UUID NOT NULL,
    Email CITEXT NOT NULL,
    Password_Hash TEXT NOT NULL,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    CONSTRAINT UQ_users_email UNIQUE (Email),
    CONSTRAINT CHK_user_email_not_blank CHECK (BTRIM(Email) <> ''),
    CONSTRAINT CHK_user_email_format CHECK (
        Email ~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$'
    ),
    CONSTRAINT CHK_password_hash_length CHECK (char_length(Password_Hash) >= 40),
    CONSTRAINT fk_users_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE
);

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_users_company_id ON users (Company_ID);
CREATE INDEX idx_users_company_email ON users (Company_ID, Email);
CREATE INDEX idx_users_company_active ON users (Company_ID) WHERE Is_Active;

-- +goose Down
DROP INDEX IF EXISTS idx_users_company_active;
DROP INDEX IF EXISTS idx_users_company_email;
DROP INDEX IF EXISTS idx_users_company_id;
DROP TABLE users;
