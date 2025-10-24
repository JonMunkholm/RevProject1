-- 021_guidance_audit.sql
-- Metadata audit trail for authoritative guidance adjustments.

-- +goose Up
create table if not exists guidance_audit (
    id uuid primary key default gen_random_uuid(),
    paragraph_id uuid not null references asc_paragraphs(id),
    change_type text not null,
    actor text not null,
    before_state jsonb,
    after_state jsonb,
    reason text,
    created_at timestamptz not null default now()
);

create index if not exists guidance_audit_paragraph_idx on guidance_audit(paragraph_id);

-- +goose Down
drop index if exists guidance_audit_paragraph_idx;
drop table if exists guidance_audit;
