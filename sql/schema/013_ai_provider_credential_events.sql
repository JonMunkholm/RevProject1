-- +goose Up
CREATE TABLE IF NOT EXISTS ai_provider_credential_events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id        uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id           uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_user_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    provider_id       text NOT NULL,
    action            text NOT NULL,
    metadata_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_provider_credential_events_company
    ON ai_provider_credential_events (company_id, provider_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_provider_credential_events_actor
    ON ai_provider_credential_events (actor_user_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_provider_credential_events;
