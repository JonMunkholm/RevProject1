-- +goose Up
DROP INDEX IF EXISTS idx_ai_provider_credentials_scope;

ALTER TABLE ai_provider_credentials
    ADD COLUMN label text,
    ADD COLUMN is_default boolean NOT NULL DEFAULT false,
    ADD COLUMN last_tested_at timestamptz,
    ADD COLUMN fingerprint text GENERATED ALWAYS AS (
        substr(encode(credential_hash, 'hex'), 1, 8)
    ) STORED;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_provider_credentials_company_default
    ON ai_provider_credentials (company_id, provider_id)
    WHERE user_id IS NULL AND is_default;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_provider_credentials_user_default
    ON ai_provider_credentials (company_id, user_id, provider_id)
    WHERE user_id IS NOT NULL AND is_default;

CREATE INDEX IF NOT EXISTS idx_ai_provider_credentials_lookup
    ON ai_provider_credentials (
        company_id,
        provider_id,
        COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid),
        is_default DESC,
        updated_at DESC,
        created_at DESC
    );

-- +goose Down
DROP INDEX IF EXISTS idx_ai_provider_credentials_lookup;
DROP INDEX IF EXISTS idx_ai_provider_credentials_user_default;
DROP INDEX IF EXISTS idx_ai_provider_credentials_company_default;

ALTER TABLE ai_provider_credentials
    DROP COLUMN IF EXISTS fingerprint,
    DROP COLUMN IF EXISTS last_tested_at,
    DROP COLUMN IF EXISTS is_default,
    DROP COLUMN IF EXISTS label;

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_provider_credentials_scope
    ON ai_provider_credentials (company_id, provider_id, COALESCE(user_id, '00000000-0000-0000-0000-000000000000'::uuid));
