# ADR-005: Retrieval Quality & Hybrid Scoring

- **Status:** Proposed (Stage 2 implementation)
- **Deciders:** Platform Engineering, Data Science
- **Technical Story:** EGRA retrieval precision roadmap
- **Related Documents:** `docs/stage2_planning.md`, `docs/embedding_system_master_blueprint_v1.0.md`, ADR‑003, ADR‑004

## Context

Stage 1 relies solely on pgvector cosine similarity for ranking search results. While that works for semantic queries, the EGRA roadmap expects higher precision for finance-grade use cases:

- Exact citation or keyword searches (e.g., “ASC 606-10-25-1”) should rank the cited paragraph first, even if semantic embeddings are noisy.
- Repeated queries should not re-hit OpenAI unnecessarily, both to reduce latency and control API costs.
- Compliance teams need confidence that weak matches are discarded or clearly flagged.

Without additional retrieval quality mechanisms we face:

- Increased OpenAI spend and latency for common queries.
- False positives when embeddings return semantically similar but irrelevant paragraphs.
- No straightforward way to tune precision/recall as the corpus grows (interpretive/internal docs, cross-topic content).

Alternatives considered:

1. **Stick with cosine-only ranking**: simplest but offers no path to tune precision or reduce costs.
2. **Move to a dedicated search engine (e.g., Elasticsearch with dense/sparse hybrid retrieval)**: powerful but introduces significant operational overhead compared with Postgres.
3. **Enhance Postgres-based retrieval with caching + hybrid scoring** (chosen): keeps infrastructure minimal while adding control levers.

## Decision

Implement a Stage 2 retrieval quality layer comprising three components:

1. **Embedding cache**  
   - Store query hash → embedding vector in Redis (or equivalent in-memory store) with a configurable TTL.  
   - Invalidate or refresh cache entries when relevant corpus changes occur (e.g., new authoritative content ingested, metadata updated).  
   - Future extension: when LLM-powered response summarisation is added, reuse the same cache namespace to store summaries keyed by (query, filters, top-results signature).

2. **Hybrid scoring in Postgres**  
   - Maintain both pgvector embeddings and a `tsvector`/BM25 column for full-text search.  
   - At query time compute cosine similarity and BM25/keyword score, then produce a weighted composite score (`score = α·cosine + β·bm25`) with α/β tuned via offline evaluation.  
   - Keep weights configurable (environment or DB table) so adjustments do not require redeploy.

3. **Thresholding and rerank policy**  
   - Reject or flag results whose cosine score falls below a minimum threshold (initially 0.40, adjustable).  
   - Monitor precision/recall metrics; if false positives persist, introduce an optional second-stage reranker (e.g., lightweight cross-encoder or rule-based citation prioritiser) applied to the top N hits.  
   - Document triggers for enabling rerank (e.g., user feedback, compliance review) and maintain the feature behind a configuration flag.

## Consequences

- ✅ **Improved precision**: Combining dense and sparse signals surfaces authoritative citations reliably.  
- ✅ **Cost & latency control**: Caching cuts redundant embedding calls; thresholding prevents low-confidence answers.  
- ✅ **Future extensibility**: Framework accommodates LLM summaries and advanced rerankers without re-architecting the stack.  
- ⚠️ **Implementation effort**: Requires Redis (or alternative cache), SQL changes, weight-tuning scripts, and monitoring dashboards.  
- ⚠️ **Operational complexity**: Cache invalidation must be managed carefully; hybrid scoring introduces additional tuning responsibility.
