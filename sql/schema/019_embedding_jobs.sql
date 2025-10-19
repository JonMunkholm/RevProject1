-- 019_embedding_jobs.sql
-- Stage 2 migration: embedding job queue metadata

create table if not exists embedding_jobs (
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
    completed_at timestamptz,
    constraint fk_embedding_jobs_paragraph
        foreign key (paragraph_id) references asc_paragraphs(id) on delete cascade
);

create index if not exists embedding_jobs_status_idx on embedding_jobs(status);
create index if not exists embedding_jobs_paragraph_idx on embedding_jobs(paragraph_id);
create index if not exists embedding_jobs_created_at_idx on embedding_jobs(created_at);

create or replace function embedding_jobs_set_updated_at()
returns trigger as $$
begin
    new.updated_at = now();
    return new;
end;
$$ language plpgsql;

drop trigger if exists trg_embedding_jobs_updated_at on embedding_jobs;
create trigger trg_embedding_jobs_updated_at
before update on embedding_jobs
for each row execute function embedding_jobs_set_updated_at();

alter table asc_paragraphs
    add column if not exists embedding_status text not null default 'pending' check (embedding_status in ('pending','processing','succeeded','failed'));

create index if not exists asc_paragraphs_embedding_status_idx on asc_paragraphs(embedding_status);

