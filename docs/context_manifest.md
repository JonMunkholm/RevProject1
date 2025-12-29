# Context Manifest — EGRA Stage 1 (LLM Reference)

**Project:** RevProject1 — Embedding Governance & Retrieval Architecture (EGRA)
**Objective:** Build a functional Stage-1 working app that ingests authoritative accounting text, embeds it, stores it in Postgres + pgvector, and retrieves semantically similar text.

---

## Core Documents

| File                                              | Purpose                                                               |
| ------------------------------------------------- | --------------------------------------------------------------------- |
| `/docs/embedding_system_master_blueprint_v1.0.md` | Full architecture, schema, and governance specification.              |
| `/docs/stage1_working_app_checklist.md`           | Defines the exact scope for this build phase.                         |
| `/docs/schema_reference.sql`                      | Schema to create `asc_paragraphs`, `asc_embeddings`, `retrieval_log`. |
| `/docs/sample_paragraph.txt`                      | Example authoritative paragraph for ingestion tests.                  |
| `.env.example`                                    | Environment variables required for DB and embedding provider.         |

---

## Stage 1 Deliverables

1. **/cmd/ingest** — CLI tool that:

   - Reads a text file.
   - Computes SHA256 hash.
   - Inserts record into `asc_paragraphs`.
   - Calls OpenAI `text-embedding-3-large`.
   - Stores vector in `asc_embeddings`.

2. **/cmd/api/search** — HTTP service that:

   - Accepts query parameter `q`.
   - Generates embedding for query.
   - Performs vector search using pgvector cosine similarity.
   - Returns JSON array of top-k results (ref + score + excerpt).

3. **/internal/database** — shared DB helpers (connect, query).
4. **/internal/retrieval** — search logic (SQL query + ranking).
5. **/internal/ai** — embedding client (OpenAI API wrapper).

---

## Key Conventions

- Language: Go 1.24
- DB: Postgres 15 + pgvector ≥ 0.5
- Env vars: `DB_URL`, `OPENAI_API_KEY`, `SCHEMA_VERSION=v1.0-2025-10-15`
- Model: `text-embedding-3-large`
- Retrieval limit: top 5 results
- Default corpus: ASC 606 authoritative text only

---

## LLM Instructions

When generating code:

- Respect the folder structure above.
- Use the schema from `schema_reference.sql`.
- Implement functions marked `[ACTIONABLE]` in the blueprint.
- Follow Go idioms: context-aware, error handling, no global state.
- Use `pgx` or `database/sql` with standard pooling.
- Return structured JSON from `/api/search`.
- Keep configuration in `.env`.

When generating documentation:

- Reference Decision IDs from `/docs/DECISIONS.md`.
- Use naming consistent with “EGRA” and “asc_paragraphs”.

---

## Out of Scope for Stage 1

- Multi-tenant roles & invitations
- Rate limits or quotas
- S3 storage abstraction
- Worker retries / queueing
- Prometheus metrics / alerts
- Audit automation
