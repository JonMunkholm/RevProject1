# AI Settings / Credential Management – Context Summary

_Last updated: 2025-10-06_

## Environment
- Repository: `RevProject1`
- Database connection in `.env`: `postgres://postgres:5Om3tH1n6@localhost:5432/rev1?sslmode=disable`
- Web UI runs at `http://localhost:8080`

## Completed Work

### Role & Auth Infrastructure
- Added migration `016_company_user_roles.sql` to create `company_user_roles`.
  - Seeds first user per company as `admin`; subsequent users default to `member`.
  - Trigger keeps the table in sync when new users are created.
- Session tokens now carry `Roles map[uuid]Role` and `CurrentRole`.
- Introduced `RequireCompanyRole(min Role)` middleware; all `/app/settings` and `/api/ai` routes wrapped with at least viewer requirement.
- `internal/contextutil/roles.go` exposes helpers (e.g. `CanManageCompanyCredentials`).

### Provider Catalog & API Enhancements
- Provider catalog now loads from Postgres (`ai_provider_catalog`), seeded with OpenAI and Gemini entries; API continues to expose `GET /api/ai/providers/catalog`.
- AI handlers validate provider IDs through the catalog loader with static fallback for safety.
- Added provider status endpoint (`/api/ai/providers/{providerID}/status`) with HTMX badge response + metrics logging.
- Credential handlers now accept both JSON and traditional form submissions, returning inline HTMX notices.
- Event list endpoint (`/api/ai/providers/{providerID}/events`) supports `action`, `scope`, `actorId` filters with safe casting.
- SQLC models regenerated after each query change (`sql/queries/ai.sql`, `internal/database/ai.sql.go`).

### Settings UI
- Replaced legacy `ai_settings_page.go` with composable templates in `app/pages/settings.templ`.
- Added shared settings shell, role-based tabs (General, Users, AI).
- AI tab uses HTMX to load provider credentials, events, and status badge; inline notices are rendered server-side.
- Supporting CSS refreshed in `app/assets/css/settings.css`.

### Optional Future Work (Documented)
- Spec for database-backed provider catalog: `docs/ai-provider-catalog-spec.md` (covers schema, migration plan, admin workflow).

### Testing & Commands
- Templates: `templ generate`
- Go tests (sandbox-friendly): `GOCACHE=$(mktemp -d) go test ./...`
- Migrations: `goose -dir sql/schema up`

## Current State / Outstanding Items
- Credential POST/TEST endpoints succeed for both JSON and form requests; events and credentials persist in DB.
- HTMX refresh now renders newly added credentials in the `/app/settings/ai` table (manual verification complete).
- Provider status call now returns HTMX badge even on failure, but the log still prints the underlying error for debugging.

## Useful Paths
- Handlers: `internal/handler/ai.go`
- Credential store: `internal/ai/credentials/sqlstore/store.go`
- Settings routes: `internal/application/settings.go`
- Templates: `app/pages/settings.templ`
- Catalog: `internal/ai/provider/catalog`

## Suggested Next Steps
1. Add regression coverage (handler/unit and templ integration tests) to guard the credential-table refresh flow.
2. Verify role gating (admin vs member/viewer) and HTMX interactions across save/test/delete/status flows.
3. Extend operational playbooks for the new DB-backed provider catalog (e.g., migrations/Taskfile commands for adding providers).

## Deferred Tasks

| Item | Description | Planned Status |
| ---- | ----------- | -------------- |
| Gemini provider hardening | Add embeddings support, rate-limit/backoff handling, and richer logging/metrics for Gemini requests. | 🧩 Backlog |
| Credential UX regression tests | Automate HTMX/UI flows for provider CRUD and status checks. | 🧩 Backlog |
| Operator runbook | Document SQL/migration workflow for adding/updating catalog entries (OpenAI, Gemini, future providers). | 🧩 Backlog |

## Customer Lookup & Ticket Creation Decisions

- **Customer lookup**: start with a lean profile (id, display name, tier, status) sourced through the internal service/repository. Apply bounded retries and then return a friendly “profile unavailable” message if the upstream stays unreachable. Log each call with redacted context for audit.
- Implementation status: `fetch_customer` tool now queries `internal/database` via `internal/ai/tool/examples.go` and is registered in `internal/application/app.go` so AI conversations can call it with company scoping enforced.
- Tool registry: chat sessions now register `list_customers` for directory lookups and the expanded `fetch_customer` tool (ID or name). Email-based queries remain future work once customer/user activity is modelled.
- Chat sessions are created lazily on the first message append, preventing empty rows; see `internal/handler/ai.go` for the new flow and use the cleanup SQL noted in `docs/ui-conversation-notes.md` if legacy shells remain.
- **Ticket creation**: call the synchronous internal ticketing service so the AI can return the canonical ticket ID/status immediately. Include an idempotency key derived from the conversation/customer context, log the request/response, and expose only the minimal confirmation payload to the user.

### Deferred options for follow-up
- Richer customer payloads (contacts, billing, health metrics) or selective field flags if future prompts demand them.
- Alternate data paths, such as direct DB queries or calling the external CRM when it is the system of record.
- Broader error-handling strategies (e.g., circuit breaker metrics) beyond the current retry-and-fallback approach.
- Centralized audit store and policy-driven rate limiting once governance requirements expand.
- Ticket queue/async submission path while keeping the synchronous interface as a compatibility layer.
- Direct SaaS ticketing integrations if an internal service is unavailable.
