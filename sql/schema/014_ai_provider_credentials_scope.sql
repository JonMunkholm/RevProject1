-- +goose Up
ALTER TABLE ai_provider_credentials
    ALTER COLUMN user_id DROP NOT NULL;

ALTER TABLE ai_provider_credentials
    DROP CONSTRAINT IF EXISTS ai_provider_credentials_user_id_fkey;

ALTER TABLE ai_provider_credentials
    ADD CONSTRAINT ai_provider_credentials_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL;

ALTER TABLE ai_provider_credentials
    DROP CONSTRAINT IF EXISTS ai_provider_credentials_company_id_user_id_provider_id_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_provider_credentials_scope
    ON ai_provider_credentials (company_id, provider_id, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- +goose Down
DROP INDEX IF EXISTS idx_ai_provider_credentials_scope;

ALTER TABLE ai_provider_credentials
    DROP CONSTRAINT IF EXISTS ai_provider_credentials_user_id_fkey;

ALTER TABLE ai_provider_credentials
    ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE ai_provider_credentials
    ADD CONSTRAINT ai_provider_credentials_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE;

ALTER TABLE ai_provider_credentials
    ADD CONSTRAINT ai_provider_credentials_company_id_user_id_provider_id_key
        UNIQUE (company_id, user_id, provider_id);
