-- +goose Up
CREATE TABLE IF NOT EXISTS ai_provider_catalog (
    id                 text PRIMARY KEY,
    label              text NOT NULL,
    icon_url           text,
    description        text,
    documentation_url  text,
    capabilities       text[] NOT NULL DEFAULT '{}',
    models             text[] NOT NULL DEFAULT '{}',
    fields             jsonb NOT NULL DEFAULT '[]'::jsonb,
    enabled            boolean NOT NULL DEFAULT TRUE,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER update_ai_provider_catalog_updated_at
    BEFORE UPDATE ON ai_provider_catalog
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

INSERT INTO ai_provider_catalog (
    id,
    label,
    icon_url,
    description,
    documentation_url,
    capabilities,
    models,
    fields,
    enabled
)
VALUES (
    'openai',
    'OpenAI',
    'https://static.openai.com/logo.png',
    'ChatGPT, GPT-4o, embeddings, and more',
    'https://platform.openai.com/docs',
    ARRAY['chat', 'completion', 'embeddings'],
    ARRAY['gpt-4o', 'gpt-4o-mini', 'gpt-3.5-turbo', 'text-embedding-3-large'],
    '[
        {"id":"apiKey","label":"API Key","type":"password","required":true,"sensitive":true,"placeholder":"sk-..."},
        {"id":"baseUrl","label":"Base URL","type":"url","placeholder":"https://api.openai.com/v1"},
        {"id":"model","label":"Default Model","type":"text","placeholder":"gpt-4o-mini"}
    ]',
    TRUE
)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
DROP TRIGGER IF EXISTS update_ai_provider_catalog_updated_at ON ai_provider_catalog;
DROP TABLE IF EXISTS ai_provider_catalog;
