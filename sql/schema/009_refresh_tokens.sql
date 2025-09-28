-- +goose Up
CREATE TABLE refresh_tokens (
    ID UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    User_ID UUID NOT NULL REFERENCES users(ID) ON DELETE CASCADE,
    Token_Hash BYTEA NOT NULL,
    Issued_IP INET,
    User_Agent TEXT,
    Created_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Updated_At TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    Expires_At TIMESTAMP NOT NULL,
    Revoked_At TIMESTAMP,
    CONSTRAINT uq_refresh_tokens_hash UNIQUE (Token_Hash)
);

CREATE INDEX idx_refresh_tokens_user_active
    ON refresh_tokens (User_ID)
    WHERE Revoked_At IS NULL;

CREATE INDEX idx_refresh_tokens_expires
    ON refresh_tokens (Expires_At);

CREATE TRIGGER update_refresh_tokens_updated_at
    BEFORE UPDATE ON refresh_tokens
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose Down
DROP TRIGGER IF EXISTS update_refresh_tokens_updated_at ON refresh_tokens;
DROP INDEX IF EXISTS idx_refresh_tokens_expires;
DROP INDEX IF EXISTS idx_refresh_tokens_user_active;
DROP TABLE IF EXISTS refresh_tokens;
