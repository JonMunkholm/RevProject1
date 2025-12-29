# ADR-004: Temporal Filters & Authority Guardrails

- **Status:** Proposed (Stage 2 rollout)
- **Deciders:** Platform Engineering, Accounting Steward
- **Technical Story:** EGRA retrieval policy hardening
- **Related Documents:** `docs/embedding_system_master_blueprint_v1.0.md`, `docs/context_manifest.md`, `docs/stage2_planning.md`

## Context

Stage 1 retrieval exposes `/api/search` without enforcing temporal or authority constraints beyond what the ingest data currently encodes. The EGRA blueprint (Section 2 & 5) mandates that default responses must be both authoritative and current:

- Default experience should exclude superseded guidance unless a reviewer explicitly opts in.
- Interpretive or internal tenant content should only appear when the caller is allowed to see it, and the ranking must continue to favour authoritative sources.
- Customers need point-in-time (“as of”) queries for audit scenarios.

Without codified guardrails we risk:

- Surfacing superseded or interpretive guidance to general users, violating the authoritative-by-default policy (ADR‑002).
- Inconsistent behaviour across services that reuse the retrieval layer.
- Complex operational workarounds to keep metadata aligned whenever new ASUs or tenant documents arrive.

Alternatives considered:

1. **Continue with ad-hoc filters**: rely on callers to pass flags and on ingest operators to hand-edit metadata. Reject due to high risk of regression and lack of audit trail.
2. **Separate indices per content tier**: replicate embeddings into separate stores (authoritative vs interpretive vs internal). Provides isolation but increases operational cost and complicates cross-tier searches.
3. **Enforce guardrails at query time with role awareness** (chosen): centralize policy in the retrieval service / SQL, using existing metadata (`source_type`, `authority_score`, `effective_date`, `superseded`, `tenant_id`, `visibility_scope`).

## Decision

Adopt a retrieval policy layer in Stage 2 with the following facets:

1. **Authoritative & temporal defaults**  
   - All baseline search requests filter `source_type = 'authoritative'`, `superseded = false`, and `effective_date <= :as_of` (default `now()`).  
   - API exposes optional parameters (`as_of`, `include_superseded`) that are rejected unless the caller’s role permits them.

2. **Role- & tenant-aware overrides**  
   - Authentication middleware attaches role/tenant claims (e.g., `role=accounting_admin`, `tenant_id=acme`).  
   - Interpretive or internal content is returned only when the role explicitly enables the corresponding flags (`include_interpretive`, `include_internal`). Such content retains lower authority weighting so authoritative paragraphs stay at the top.  
   - Tenant-owned documents additionally require `tenant_id` match or inclusion within `visibility_scope`; row-level-security (RLS) policies are introduced in Postgres as defence in depth.

3. **Metadata governance workflow**  
   - Establish an accounting-owned process (admin UI or controlled scripts) to update `effective_date`, `superseded`, and related metadata whenever new guidance or internal policy changes ship.  
   - Persist an audit trail (timestamp, actor, previous/new values) for all metadata changes impacting guardrail logic.

## Consequences

- ✅ **Policy compliance**: Default responses honour ADR‑002 (“authoritative by default”) and fulfil the temporal retrieval requirements from the master blueprint.
- ✅ **Consistent behaviour**: All services using the retrieval layer inherit the same guardrails; overrides require explicit role grants.
- ✅ **Auditability**: Metadata changes and opt-in overrides are tracked, supporting external audits and internal reviews.
- ⚠️ **Implementation effort**: Requires auth middleware, role mapping, SQL updates, potential RLS policies, and metadata tooling.
- ⚠️ **Operational overhead**: Accounting stewards must maintain metadata currency; failures to update `superseded` flags could still leak stale content (mitigated by workflow automation).
