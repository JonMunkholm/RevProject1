-- +goose Up
-- Conversation sessions are scoped to a company so every coworker can access them.
CREATE TABLE IF NOT EXISTS ai_conversation_sessions (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id   uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id  text NOT NULL,
    title        text,
    metadata     jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversation_sessions_company
    ON ai_conversation_sessions (company_id, created_at DESC);

-- Individual conversation turns linked to a session.
CREATE TABLE IF NOT EXISTS ai_conversation_messages (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id  uuid NOT NULL REFERENCES ai_conversation_sessions (id) ON DELETE CASCADE,
    role        text NOT NULL,
    content     text NOT NULL,
    metadata    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversation_messages_session
    ON ai_conversation_messages (session_id, created_at ASC);

-- +goose Down
DROP TABLE IF EXISTS ai_conversation_messages;
DROP TABLE IF EXISTS ai_conversation_sessions;
