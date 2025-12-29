# ADR-002: Authoritative-by-Default Retrieval Policy

- **Status:** Accepted (2025-10-06)
- **Deciders:** Accounting Stewardship, Platform Engineering, Product AI
- **Technical Story:** EGRA retrieval UX & compliance requirements
- **Related Documents:** `docs/embedding_system_master_blueprint_v1.0.md`, `docs/ai-credential-management-spec.md`

## Context

EGRA must prioritise authoritative accounting guidance (e.g., ASC 606) while allowing interpretive or internal documents in future phases. Early customer pilots emphasised:

- Regulatory risk if non-authoritative content surfaces without explicit opt-in.
- Need for auditors to verify provenance and replay retrieval sessions.
- Role-based controls (viewer/member/admin) driving what content is visible.

Alternatives considered:

- **Equal weighting of all sources:** Simplifies indexing but breaches compliance and undermines trust.
- **Per-tenant defaults:** Adds configuration complexity before we have multi-tenant maturity.

## Decision

Set the retrieval stack to **authoritative-only by default**:

- `/api/search` and the Stage 1 CLI default to authoritative paragraphs (index role `authoritative_current`).
- Interpretive/internal corpora require explicit query parameters and capability checks (to be implemented in later phases).
- HTMX settings UI communicates scope and requires elevated roles for company-wide credentials.

Templates, handlers, and documentation now clearly label authoritative content and outline escalation paths for other tiers.

## Consequences

- ✅ **Compliance alignment:** Primary use cases surface authoritative guidance only, satisfying accounting stewardship requirements.
- ✅ **Clear UX:** Users understand when they leave authoritative scope, reducing accidental misuse.
- ✅ **Audit-ready:** Retrieval logs can assume authoritative precedence unless flags indicate otherwise.
- ⚠️ **Configuration overhead later:** Future phases must introduce switches to include interpretive/internal corpora without regressing defaults.
- ⚠️ **Content freshness:** Authoritative-only mode relies on timely ingest of ASC updates; backlog tracks automation for new releases.
