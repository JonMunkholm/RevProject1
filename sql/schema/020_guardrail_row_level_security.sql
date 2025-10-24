-- 020_guardrail_row_level_security.sql
-- Establish row-level security and helper functions for retrieval guardrails.

-- +goose Up
create schema if not exists app_guardrails;

-- +goose StatementBegin
create or replace function app_guardrails.app_guardrails_bool(setting text, default_value boolean)
returns boolean
language sql
stable
as $$
    select coalesce(nullif(current_setting(setting, true), '')::boolean, $2)
$$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function app_guardrails.app_guardrails_tenant()
returns text
language sql
stable
as $$
    select nullif(current_setting('app.guardrails.tenant_id', true), '')
$$;
-- +goose StatementEnd

-- +goose StatementBegin
create or replace function app_guardrails.app_guardrails_can_read_paragraph(p_source text, p_superseded boolean, p_tenant text)
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
$$;
-- +goose StatementEnd

alter table asc_paragraphs enable row level security;
alter table asc_paragraphs force row level security;
alter table asc_embeddings enable row level security;
alter table asc_embeddings force row level security;

drop policy if exists guardrails_paragraphs_select on asc_paragraphs;
create policy guardrails_paragraphs_select on asc_paragraphs
    for select
    using (app_guardrails.app_guardrails_can_read_paragraph(source_type, superseded, tenant_id));

drop policy if exists guardrails_embeddings_select on asc_embeddings;
create policy guardrails_embeddings_select on asc_embeddings
    for select
    using (
        exists (
            select 1
            from asc_paragraphs p
            where p.id = asc_embeddings.paragraph_id
              and app_guardrails.app_guardrails_can_read_paragraph(p.source_type, p.superseded, p.tenant_id)
        )
    );

-- +goose Down
drop policy if exists guardrails_embeddings_select on asc_embeddings;
drop policy if exists guardrails_paragraphs_select on asc_paragraphs;

alter table asc_embeddings disable row level security;
alter table asc_embeddings no force row level security;
alter table asc_paragraphs disable row level security;
alter table asc_paragraphs no force row level security;

drop function if exists app_guardrails.app_guardrails_can_read_paragraph(text, boolean, text);
drop function if exists app_guardrails.app_guardrails_tenant();
drop function if exists app_guardrails.app_guardrails_bool(text, boolean);
drop schema if exists app_guardrails cascade;
