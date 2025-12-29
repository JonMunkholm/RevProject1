# Embedding System Master Blueprint (EGRA)
**Version:** v1.0  
**Effective Date:** 2025-10-15  
**Owner:** Accounting (Authoritative Steward) + Platform Owner (AI)  
**Scope:** US GAAP revenue topics — **ASC 606** first, with planned expansion to closely related topics (ASC 340-40, 610-20, selected 842/805/820 cross-refs).  

> **Purpose:** A finance-grade, temporally faithful, auditable guidance system that defaults to **authoritative US GAAP** and can incorporate interpretive and tenant internal policy content without compromising precedence.

---

## Table of Contents
1. Executive Frame (Goal, Guardrails, Operating Model)  
2. Corpus & Authority Model  
3. Metadata & Schema (Postgres + pgvector)  
4. Ingestion & Re-embedding Pipeline  
5. Temporal Retrieval Policy (Default & As-Of)  
6. Indexing & Retrieval Architecture  
7. Governance, RACI & Change Control  
8. CI / Test / Observability  
9. Logging, Replay & Auditability  
10. Quality, Drift & Gold Tests  
11. Security, Access & Licensing  
12. Operations, Backups & Lifecycle  
13. Roadmap (Phases 0–3)  
14. Interfaces & Code Seams (LLM-Ready)  
15. Appendices (Manifests, .env, ADRs)  
16. Optional & Future Enhancements  

---

## 1. Executive Frame
**Goal:** Finance-grade, temporally faithful, auditable guidance system for US GAAP revenue recognition.  
**Guardrails:** (i) **Authoritative-by-default**, (ii) point-in-time retrieval, (iii) explainability & replay, (iv) licensing & IP compliance.  
**Operating model:** Decisions captured in **Decision Log** (ADRs). Enforced via tests, CI gates, and audits.

[POLICY] The system must never surface superseded content in default mode; interpretive or internal policy content requires explicit opt-in and is always **subordinate** to authoritative content in ranking and display.

[OWNER] Accounting (Authoritative Steward) + Platform Owner.

[LLM EXECUTION CHECKLIST]
- Ensure default queries apply authoritative filters.  
- Build retrieval pipeline with temporal knob.  
- Include immutable logging and replay tool.

---

## 2. Corpus & Authority Model
### 2.1 Tiers & Precedence
- **Tier 1 — Authoritative (default):** FASB ASC **606** complete; plan to add **340-40**, **610-20**, and selected **842/805/820** cross-refs for scope screening and context.  
- **Tier 2 — Interpretive (opt-in):** Big-4 / AICPA interpretations; stored in separate index with lower `authority_score`.  
- **Tier 3 — Internal (tenant):** Tenant policy documents uploaded by customers; scope-isolated with `visibility_scope` & `tenant_id`.

**Precedence:** `authoritative > interpretive > internal`.

[POLICY] Default retrieval = authoritative & current only. Interpretive/internal must be explicitly requested by role/call-site.

[TEST]
- Mixed corpus query returns authoritative first.  
- Superseded content never appears without `include_superseded=true`.

[OWNER] Accounting (policy); Data (ingest correctness).

[LLM EXECUTION CHECKLIST]
- Encode `source_type`, `authority_score`, `tenant_id`, `visibility_scope` fields.  
- Build opt-in switches for interpretive/internal retrieval.

---

## 3. Metadata & Schema (Postgres + pgvector)
### 3.1 Core, Operational, Semantic & Governance Fields
**Core**: `framework`, `topic`, `asc_reference`, `guidance_version` (ASU id), `effective_date`, `issued_date`, `superseded`, `supersedes`, `superseded_by`, `source_type`, `authority_score`.  
**Operational**: `schema_version`, `embedding_model`, `embedding_date`, `source_id` (SHA256), `checksum`, `ingested_by`, `job_id`, `license_id`.  
**Semantic (optional)**: `amends_topics[]`, `related_paragraphs[]`, `cross_refs[]`, `step_model_ref`, `topic_tags[]`, `keywords[]`, `risk_flags[]`.  
**Governance**: `data_sensitivity`, `visibility_scope[]`, `tenant_id`, `policy_id`.

### 3.2 Authoritative Units (Paragraph-Level)
```sql
create table if not exists asc_paragraphs (
  id uuid primary key,
  framework text not null default 'US_GAAP',
  topic text not null,
  asc_reference text not null,
  guidance_version text not null,
  issued_date date,
  effective_date date,
  early_adoption_allowed boolean,
  supersedes text,
  superseded_by text,
  superseded boolean default false,
  amends_topics text[],
  related_paragraphs text[],
  cross_refs text[],
  step_model_ref text,
  source_type text check (source_type in ('authoritative','interpretive','internal')),
  authority_score numeric default 1.0,
  source_id text,
  checksum text,
  license_id text,
  data_sensitivity text default 'public',
  visibility_scope text[] default '{public}',
  tenant_id text,
  policy_id text,
  schema_version text not null,
  content text not null,
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);
```

### 3.3 Embeddings (Versioned)
```sql
create extension if not exists vector;
create table if not exists asc_embeddings (
  id uuid primary key,
  paragraph_id uuid not null references asc_paragraphs(id) on delete cascade,
  embedding vector(3072) not null,
  embedding_model text not null,
  embedding_date timestamptz not null,
  index_role text default 'authoritative_current',
  schema_version text not null,
  created_by text,
  created_at timestamptz default now()
);
```

### 3.4 Retrieval Log (Immutable)
```sql
create table if not exists retrieval_log (
  id bigserial primary key,
  ts timestamptz default now(),
  actor text,
  query text,
  as_of_date date,
  filters jsonb,
  model_id text,
  top_k int,
  results jsonb,
  response_hash text,
  immutable boolean default true
);
```
