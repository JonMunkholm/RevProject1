-- +goose Up
CREATE TABLE users (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    User_Name CITEXT NOT NULL,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Company_ID UUID NOT NULL,
    Is_Active BOOL NOT NULL DEFAULT TRUE,
    CONSTRAINT UQ_users_company_name UNIQUE (Company_ID, User_Name),
    CONSTRAINT CHK_user_name_not_blank CHECK (BTRIM(User_Name) <> ''),
    CONSTRAINT fk_users_company_id
        FOREIGN KEY (Company_ID)
        REFERENCES companies(ID)
        ON DELETE CASCADE
);

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE INDEX idx_users_company_id ON users (Company_ID);
CREATE INDEX idx_users_company_active ON users (Company_ID) WHERE Is_Active;

-- +goose Down
DROP INDEX IF EXISTS idx_users_company_active;
DROP INDEX IF EXISTS idx_users_company_id;
DROP TABLE users;
