-- ============================================================================
--  Embedding Governance & Retrieval Architecture (EGRA)
--  Schema Reference — Stage 1 Baseline
--  Effective: 2025-10-15   Schema Version: v1.0-2025-10-15
-- ============================================================================

-- Ensure pgvector extension is available.
create extension if not exists vector;

-- ----------------------------------------------------------------------------
--  Table: asc_paragraphs
--  Authoritative and interpretive paragraph-level text units.
-- ----------------------------------------------------------------------------
create table if not exists asc_paragraphs (
  id uuid primary key,
  framework text not null default 'US_GAAP',          -- e.g. 'US_GAAP', 'IFRS'
  topic text not null,                                -- e.g. 'ASC606'
  asc_reference text not null,                        -- e.g. 'ASC606-10-25-1'
  guidance_version text not null,                     -- e.g. 'ASU2021-08'
  issued_date date,
  effective_date date,
  early_adoption_allowed boolean,
  supersedes text,
  superseded_by text,
  superseded boolean default false,
  amends_topics text[],                               -- topics amended by this paragraph
  related_paragraphs text[],                          -- cross-refs within ASC
  cross_refs text[],                                  -- external cross-references
  step_model_ref text,                                -- e.g. 'Step3-DetermineTransactionPrice'
  source_type text check (source_type in ('authoritative','interpretive','internal')),
  authority_score numeric default 1.0,
  source_id text,                                     -- SHA256 fingerprint
  checksum text,                                      -- optional raw checksum
  license_id text,
  data_sensitivity text default 'public',             -- 'public'|'confidential'|'restricted'
  visibility_scope text[] default '{public}',         -- e.g. '{"tenant:acme"}'
  tenant_id text,                                     -- for internal policies
  policy_id text,                                     -- internal policy grouping
  schema_version text not null,                       -- e.g. 'v1.0-2025-10-15'
  content text not null,                              -- authoritative or licensed text
  created_at timestamptz default now(),
  updated_at timestamptz default now()
);

comment on table asc_paragraphs is
  'Authoritative and interpretive accounting guidance paragraphs (ASC, policies).';

-- ----------------------------------------------------------------------------
--  Table: asc_embeddings
--  Vector representations of paragraph text.
-- ----------------------------------------------------------------------------
create table if not exists asc_embeddings (
  id uuid primary key,
  paragraph_id uuid not null references asc_paragraphs(id) on delete cascade,
  embedding vector(3072) not null,                    -- matches text-embedding-3-large
  embedding_model text not null,                      -- e.g. 'text-embedding-3-large@2025-01'
  embedding_date timestamptz not null,
  index_role text default 'authoritative_current',    -- 'authoritative_current'|'archive'|'interpretive'
  schema_version text not null,
  created_by text,                                    -- system/service id
  created_at timestamptz default now()
);

comment on table asc_embeddings is
  'Vector embeddings for authoritative and interpretive paragraphs.';

-- ----------------------------------------------------------------------------
--  Table: retrieval_log
--  Immutable log of all retrieval requests for audit/replay.
-- ----------------------------------------------------------------------------
create table if not exists retrieval_log (
  id bigserial primary key,
  ts timestamptz default now(),
  actor text,                                         -- user/service identifier
  query text,                                         -- raw search query
  as_of_date date,                                   -- point-in-time parameter
  filters jsonb,                                     -- e.g. { "include_superseded": false }
  model_id text,                                     -- embedding model used
  top_k int,                                         -- number of results returned
  results jsonb,                                     -- refs + scores + metadata snapshot
  response_hash text,                                -- hash of LLM/response payload
  immutable boolean default true
);

comment on table retrieval_log is
  'Immutable audit log of retrieval operations (query, filters, top-k, hash).';

-- ----------------------------------------------------------------------------
--  Helpful Indexes
-- ----------------------------------------------------------------------------
-- Paragraph text search and metadata filters.
create index if not exists idx_paragraphs_ref
  on asc_paragraphs(asc_reference);

create index if not exists idx_paragraphs_current
  on asc_paragraphs((not superseded));

create index if not exists idx_paragraphs_visibility
  on asc_paragraphs using gin (visibility_scope);

create index if not exists idx_paragraphs_content_fts
  on asc_paragraphs using gin (to_tsvector('english', content));

-- Vector search index for cosine similarity (IVFFlat).
create index if not exists idx_embeddings_vector
  on asc_embeddings using ivfflat (embedding vector_cosine_ops)
  with (lists = 200);

-- ----------------------------------------------------------------------------
--  Basic Constraints / Integrity Checks
-- ----------------------------------------------------------------------------
alter table asc_paragraphs
  add constraint asc_paragraphs_effective_not_future
  check (effective_date is null or effective_date <= current_date)
  not valid;

-- ----------------------------------------------------------------------------
--  Views (Optional Helpers)
-- ----------------------------------------------------------------------------
create or replace view v_current_authoritative as
  select *
  from asc_paragraphs
  where superseded = false and source_type = 'authoritative';

-- ----------------------------------------------------------------------------
--  Schema Version Record
-- ----------------------------------------------------------------------------
insert into asc_paragraphs (
  id, framework, topic, asc_reference, guidance_version,
  issued_date, effective_date, source_type, authority_score,
  source_id, schema_version, content
)
values (
  gen_random_uuid(), 'META', 'SCHEMA', 'SCHEMA-V1.0', '2025-10-15',
  current_date, current_date, 'internal', 1.0,
  'schema-v1.0-checksum', 'v1.0-2025-10-15', 'Schema initialization record.'
)
on conflict do nothing;

-- ============================================================================
--  End of Schema Reference v1.0
-- ============================================================================
