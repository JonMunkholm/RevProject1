-- +goose Up
-- AI provider credentials store per-user LLM secrets and metadata
CREATE TABLE IF NOT EXISTS ai_provider_credentials (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id         uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id            uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id        text NOT NULL,
    credential_cipher  bytea NOT NULL,
    credential_hash    bytea NOT NULL,
    metadata           jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    last_used_at       timestamptz,
    rotated_at         timestamptz,
    UNIQUE (company_id, user_id, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_ai_provider_credentials_hash
    ON ai_provider_credentials (company_id, provider_id, credential_hash);

CREATE INDEX IF NOT EXISTS idx_ai_provider_credentials_user
    ON ai_provider_credentials (company_id, user_id);

-- AI user preferences for default providers/models
CREATE TABLE IF NOT EXISTS ai_user_preferences (
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    company_id   uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    provider_id  text NOT NULL,
    model        text,
    metadata     jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (company_id, user_id)
);

-- Tool invocation audit log for observability and debugging
CREATE TABLE IF NOT EXISTS ai_tool_invocations (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid REFERENCES users (id) ON DELETE SET NULL,
    provider_id    text NOT NULL,
    tool_name      text NOT NULL,
    status         text NOT NULL DEFAULT 'success',
    request        jsonb,
    response       jsonb,
    error_message  text,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_tool_invocations_user
    ON ai_tool_invocations (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_tool_invocations_provider
    ON ai_tool_invocations (provider_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_tool_invocations;
DROP TABLE IF EXISTS ai_user_preferences;
DROP TABLE IF EXISTS ai_provider_credentials;
