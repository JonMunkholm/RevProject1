-- +goose Up
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
    'gemini',
    'Google Gemini',
    'https://ai.google.dev/static/images/share.png',
    'Multimodal models for chat, reasoning, and embeddings',
    'https://ai.google.dev/gemini-api/docs',
    ARRAY['chat', 'completion', 'embeddings', 'reasoning'],
    ARRAY['gemini-pro', 'gemini-pro-vision', 'text-embedding-004'],
    '[
        {"id":"apiKey","label":"API Key","type":"password","required":true,"sensitive":true,"placeholder":"AIza..."},
        {"id":"projectId","label":"Project ID","type":"text","placeholder":"my-project"},
        {"id":"baseUrl","label":"Base URL","type":"url","placeholder":"https://generativelanguage.googleapis.com/v1beta"}
    ]',
    TRUE
)
ON CONFLICT (id) DO UPDATE SET
    label = EXCLUDED.label,
    icon_url = EXCLUDED.icon_url,
    description = EXCLUDED.description,
    documentation_url = EXCLUDED.documentation_url,
    capabilities = EXCLUDED.capabilities,
    models = EXCLUDED.models,
    fields = EXCLUDED.fields,
    enabled = EXCLUDED.enabled,
    updated_at = now();

-- +goose Down
DELETE FROM ai_provider_catalog WHERE id = 'gemini';
