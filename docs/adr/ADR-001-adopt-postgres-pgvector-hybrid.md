# ADR-001: Adopt Postgres + pgvector Hybrid Storage

- **Status:** Accepted (2025-10-05)
- **Deciders:** Platform Engineering, Data Platform, Product AI
- **Technical Story:** EGRA Stage 1 ingestion & retrieval baseline
- **Related Documents:** `docs/embedding_system_master_blueprint_v1.0.md`, `docs/stage1_working_app_checklist.md`

## Context

Stage 1 of EGRA requires a single persistence layer that can:

1. Store authoritative accounting paragraphs with rich metadata.
2. Persist high-dimensional embedding vectors for semantic search.
3. Support transactional ingest while keeping infrastructure lightweight for early prototyping.

Alternatives considered:

- **Dual data stores (Postgres + dedicated vector DB):** Higher operational overhead, additional networking and consistency complexities.
- **Managed proprietary vector service:** Locks us into a vendor before requirements are fully understood and complicates local development.
- **File-based artifacts:** Insufficient for transactional ingest, lacks query capabilities, hard to secure/audit.

## Decision

Use **Postgres 15** as the system of record and enable the **pgvector** extension for similarity search. The initial schema creates:

- `asc_paragraphs` table for authoritative content and metadata.
- `asc_embeddings` table with `vector(3072)` column storing OpenAI `text-embedding-3-large` vectors, indexed via IVFFlat.

The ingest CLI (`cmd/ingest`) and retrieval service (`cmd/api`) use this schema directly. SQLC-generated queries provide typed access for Go services, aligning with existing tooling.

## Consequences

- ✅ **Unified stack:** One database to operate for both structured and vector data; fits existing Go/Postgres expertise.
- ✅ **Local parity:** Developers can spin up Postgres + pgvector locally; Stage 1 CLI/API verified against this stack.
- ✅ **Auditable:** Storing vectors in Postgres keeps change history with standard audit tooling and transaction logs.
- ⚠️ **Performance limits:** pgvector is adequate for Stage 1 scale but may require tuning or sharding as corpus size grows; future ADRs can revisit external vector services.
- ⚠️ **Extension dependency:** Environments must support installing `pgvector`; documentation now includes installation prerequisites.
