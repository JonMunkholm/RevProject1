-- +goose Up
-- Document processing job table for AI-based parsing, summarization, or classification tasks.
CREATE TABLE IF NOT EXISTS ai_document_jobs (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id    uuid NOT NULL REFERENCES companies (id) ON DELETE CASCADE,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider_id   text NOT NULL,
    status        text NOT NULL DEFAULT 'queued',
    request       jsonb NOT NULL,
    response      jsonb,
    error_message text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    completed_at  timestamptz
);

CREATE INDEX IF NOT EXISTS idx_ai_document_jobs_company
    ON ai_document_jobs (company_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS ai_document_jobs;
