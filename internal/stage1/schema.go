package stage1

import (
	"context"
	"database/sql"
)

var bootstrapStatements = []string{
	`create extension if not exists vector`,
	`create extension if not exists pgcrypto`,
	`create table if not exists asc_paragraphs (
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
		updated_at timestamptz default now(),
		embedding_status text not null default 'pending' check (embedding_status in ('pending','processing','succeeded','failed'))
	)`,
	`create table if not exists asc_embeddings (
		id uuid primary key,
		paragraph_id uuid not null references asc_paragraphs(id) on delete cascade,
		embedding vector(3072) not null,
		embedding_model text not null,
		embedding_date timestamptz not null,
		index_role text default 'authoritative_current',
		schema_version text not null,
		created_by text,
		created_at timestamptz default now()
	)`,
	`create table if not exists retrieval_log (
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
	)`,
	`create index if not exists idx_paragraphs_source_id on asc_paragraphs(source_id)`,
	`create index if not exists idx_embeddings_paragraph on asc_embeddings(paragraph_id)`,
	`create index if not exists asc_paragraphs_embedding_status_idx on asc_paragraphs(embedding_status)`,
	`create table if not exists embedding_jobs (
		id uuid primary key default gen_random_uuid(),
		paragraph_id uuid not null references asc_paragraphs(id) on delete cascade,
		status text not null default 'pending' check (status in ('pending','in_progress','succeeded','failed','dead_letter')),
		attempts int not null default 0,
		last_error text,
		source_hash text not null,
		model text not null,
		priority text not null default 'normal',
		metadata_version text not null,
		created_at timestamptz not null default now(),
		updated_at timestamptz not null default now(),
		completed_at timestamptz
	)`,
	`create index if not exists embedding_jobs_status_idx on embedding_jobs(status)`,
	`create index if not exists embedding_jobs_paragraph_idx on embedding_jobs(paragraph_id)`,
	`create index if not exists embedding_jobs_created_at_idx on embedding_jobs(created_at)`,
	`create or replace function embedding_jobs_set_updated_at()
returns trigger as $$
begin
	new.updated_at = now();
	return new;
end;
$$ language plpgsql`,
	`drop trigger if exists trg_embedding_jobs_updated_at on embedding_jobs`,
	`create trigger trg_embedding_jobs_updated_at
before update on embedding_jobs
for each row execute function embedding_jobs_set_updated_at()`,
	`create schema if not exists app_guardrails`,
	`create or replace function app_guardrails.app_guardrails_bool(setting text, default_value boolean)
returns boolean
language sql
stable
as $$
	select coalesce(nullif(current_setting(setting, true), '')::boolean, $2)
$$`,
	`create or replace function app_guardrails.app_guardrails_tenant()
returns text
language sql
stable
as $$
	select nullif(current_setting('app.guardrails.tenant_id', true), '')
$$`,
	`create or replace function app_guardrails.app_guardrails_can_read_paragraph(p_source text, p_superseded boolean, p_tenant text)
returns boolean
language sql
stable
as $$
	select case
		when coalesce(p_source, '') = 'authoritative'
			then (app_guardrails.app_guardrails_bool('app.guardrails.include_superseded', false) or p_superseded = false)
		when coalesce(p_source, '') = 'interpretive'
			then app_guardrails.app_guardrails_bool('app.guardrails.include_interpretive', false)
		when coalesce(p_source, '') = 'internal'
			then app_guardrails.app_guardrails_bool('app.guardrails.include_internal', false)
				and coalesce(p_tenant, '') = coalesce(app_guardrails.app_guardrails_tenant(), '')
		else false
	end
$$`,
	`alter table asc_paragraphs enable row level security`,
	`alter table asc_paragraphs force row level security`,
	`alter table asc_embeddings enable row level security`,
	`alter table asc_embeddings force row level security`,
	`drop policy if exists guardrails_paragraphs_select on asc_paragraphs`,
	`create policy guardrails_paragraphs_select on asc_paragraphs
for select
using (app_guardrails.app_guardrails_can_read_paragraph(source_type, superseded, tenant_id))`,
	`drop policy if exists guardrails_embeddings_select on asc_embeddings`,
	`create policy guardrails_embeddings_select on asc_embeddings
for select
using (
	exists (
		select 1
		from asc_paragraphs p
		where p.id = asc_embeddings.paragraph_id
		  and app_guardrails.app_guardrails_can_read_paragraph(p.source_type, p.superseded, p.tenant_id)
	)
)`,
	`create table if not exists guidance_audit (
		id uuid primary key default gen_random_uuid(),
		paragraph_id uuid not null references asc_paragraphs(id) on delete cascade,
		change_type text not null,
		actor text not null,
		before_state jsonb,
		after_state jsonb,
		reason text,
		created_at timestamptz not null default now()
	)`,
	`create index if not exists guidance_audit_paragraph_idx on guidance_audit(paragraph_id)`,
}

// EnsureSchema creates the Stage 1 tables and indexes if they do not exist.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range bootstrapStatements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
