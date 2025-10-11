package stage1

import (
	"context"
	"database/sql"
)

var bootstrapStatements = []string{
	`create extension if not exists vector`,
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
		updated_at timestamptz default now()
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
