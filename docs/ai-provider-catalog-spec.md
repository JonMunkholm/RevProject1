# AI Provider Catalog Externalisation – Spec

## Overview

The current AI settings UI renders provider metadata (labels, fields, documentation links) from static code in `internal/ai/provider/catalog`. This lightweight approach is fine while only OpenAI is supported, but adding more providers will require frequent deploys to tweak field definitions, change doc links, or enable/disable options per environment. To minimize friction, we can move the catalog to a data-driven model.

## Goals

- Allow operators to add, modify, or retire providers without code changes or redeploys.
- Support environment-specific overrides (e.g. staging vs production supporting different providers).
- Continue serving the catalog to the UI via `GET /api/ai/providers/catalog` and enforcing validation in backend handlers.

## Non-Goals

- We will not build a fully user-editable provider management UI in this phase; updates can be delivered via migrations or admin tooling.
- We do not tackle dynamic credential logic (e.g. provider-specific validation) yet; we still rely on Go code once provider IDs are known.

## Design Options

1. **Config file (YAML/JSON) loaded on startup.**
   - Pros: simple, no schema changes, easy to version in Git.
   - Cons: still requires restarts to apply changes, awkward to vary per environment without branching.

2. **Database-backed catalog (recommended).**
   - Introduce a `ai_provider_catalog` table (id, label, icon_url, description, documentation_url, capabilities, models, fields JSONB, enabled flag).
   - Add a companion table for provider field definitions if we want structured rows; otherwise keep a JSONB array of fields.
   - Seed defaults via migration. Operators can override rows with SQL or small admin CLI.
   - `internal/ai/provider/catalog` becomes a thin layer that loads from DB (with fallback to built-in defaults if desired).

## Proposed Schema (Option 2)

```
CREATE TABLE ai_provider_catalog (
    id TEXT PRIMARY KEY,
    label TEXT NOT NULL,
    icon_url TEXT,
    description TEXT,
    documentation_url TEXT,
    capabilities TEXT[] NOT NULL DEFAULT '{}',
    models TEXT[] NOT NULL DEFAULT '{}',
    fields JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER update_ai_provider_catalog_updated_at
BEFORE UPDATE ON ai_provider_catalog
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

- `fields` JSON structure mirrors the current Go struct: `[ {"id": "apiKey", "label": "API Key", "type": "password", ... } ]`.
- Optionally split fields into a separate table if we want relational constraints.

## Backend Changes

- Extend SQLC queries to read the catalog from the database, with `enabled = TRUE` filtering.
- Add caching or in-memory reload to avoid excess DB hits (lazy load with TTL or startup preload).
- Provide a fallback map in Go if no records exist (e.g. bootstrap new deployments).

## Migration Plan

1. Create migration adding the catalog table and seed rows for existing providers.
2. Update `internal/ai/provider/catalog` to first try DB; if empty, load default struct and optionally insert to DB.
3. Adjust `README`/docs so operators know how to add providers via SQL.

## Future Work

- Build internal admin UI to manage providers.
- Allow environment-specific overrides via additional columns (e.g. `environment`, `company_id` overrides).
- Support provider-specific validation rules stored alongside catalog entries.
