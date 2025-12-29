# Decision Log

The log captures material architecture and governance choices for the Embedding Governance & Retrieval Architecture (EGRA) programme. Each entry links to a detailed Architecture Decision Record (ADR) and notes the current implementation state.

| Date       | Decision                                                                   | Status     | ADR                                | Notes |
| ---------- | -------------------------------------------------------------------------- | ---------- | ---------------------------------- | ----- |
| 2025-10-05 | Adopt Postgres + pgvector hybrid storage for embeddings and paragraph data | ✅ Adopted | [ADR-001](adr/ADR-001-adopt-postgres-pgvector-hybrid.md) | Stage 1 ingest/search paths now persist authoritative content in Postgres with pgvector similarity queries. |
| 2025-10-06 | Enforce authoritative-by-default retrieval policy for customer queries     | ✅ Adopted | [ADR-002](adr/ADR-002-authoritative-by-default-retrieval-policy.md) | Retrieval API & templates default to authoritative corpus; interpretive/internal content requires explicit opt-in. |
| 2025-10-08 | Introduce asynchronous embedding worker queue for future scale             | 🧩 Planned | [ADR-003](adr/ADR-003-asynchronous-embedding-worker-queue.md) | Planned for Stage 2 once synchronous ingest baseline is stable; backlog item tracks queue implementation. |
