# AI Credential Management – Implementation Spec

_Status: current as of 2025-10-06_

## Scope
This document captures the existing construction and behaviour of the AI credential
management stack. It covers credential storage, permission model, HTTP/HTMX
interaction patterns, and the Settings › AI user experience. Migrations and
operational runbooks are out of scope for this summary.

## Architecture Overview
- **Entry points**
  - UI: `/app/settings/ai` (templ components rendered by
    `internal/application/settings.go`).
  - API: `/api/ai/*` handlers in `internal/handler/ai.go`.
- **Session context**: JWT middleware (`internal/auth/authMiddleware.go`) populates
  `auth.Session` with `CurrentRole` and capability flags that drive downstream
  authorization and conditional rendering.
- **Credential persistence**: Implemented through SQLC store
  (`internal/ai/credentials/sqlstore/store.go`) and decrypted on demand by the
  DB resolver (`internal/ai/credentials/dbresolver/resolver.go`).
- **Templates**: Generated from `app/pages/settings.templ` →
  `app/pages/settings_templ.go`, with supporting table helpers in
  `app/pages/settings_tables.go`.

## Data Model
- Table `ai_provider_credentials`
  - Key columns: `company_id`, `provider_id`, `user_id` (nullable for
    company-level credentials).
  - Secret storage: `credential_cipher` (AES-encrypted API key),
    `credential_hash` (SHA-256 of plaintext), generated `fingerprint`
    (first 8 hex chars of the hash).
  - Metadata JSON: stores provider-specific fields plus
    `metadata["key_suffix"]`, the last four characters of the plaintext key.
  - Flags: `is_default`, `last_tested_at`, timestamps for created/updated/usage.
- Lookup helpers: SQL queries in `sql/queries/ai.sql` support listing by company,
  provider scope, resolver ordering, and default clearing.
- Resolver flow: `ai.NewDBCredentialResolver` decrypts secrets when the AI client
  (conversation/document services) requests a credential. No plaintext is written
  back to persistence.

## Permission Model
- Role hierarchy (`internal/auth/auth.go`): viewer < member < admin.
- JWT middleware (`internal/auth/authMiddleware.go`) maps roles to capabilities:
  - `CanViewProviderCredentials`: viewer and above.
  - `CanManagePersonalCredentials`: member and above.
  - `CanManageCompanyCredentials`: admin only.
- Settings routing enforces `RequireCompanyRole(RoleViewer)` for `/app/settings/*`
  and checks capabilities before rendering AI content (via
  `contextutil.CanViewProviderCredentials`).
- Handler enforcement (`internal/handler/ai.go`):
  - `canManageCredentialScope` ensures only authorised sessions can insert,
    update, or delete user/company scoped records.
  - Event endpoints share the same viewer gate; mutations additionally require
    personal/company manage capability and scope ownership.
- Role provisioning: migration `016_company_user_roles.sql` seeds each company’s
  first user as admin and keeps the `company_user_roles` table in sync; this is
  the source of truth read during login when issuing session tokens.

## API & HTMX Endpoints
All routes live under `/api/ai` and expect the session context described above.

| Endpoint | Verb | Purpose | Sample Request | Sample Response |
|----------|------|---------|----------------|-----------------|
| `/api/ai/providers/{provider}/credentials` | GET | List credentials for active company. When `provider` is set, returns both company and current-user scopes unless explicit `scope`/`userId` query filters are supplied. | `GET /api/ai/providers/openai/credentials?limit=20` | `200 OK` JSON body: `{ "items": [{"id":"…","scope":"company","fingerprint":"1a2b3c4d","keySuffix":"XYZ9",…}], "nextOffset":0 }` or HTMX partial table when `HX-Request: true`. |
| `/api/ai/providers/{provider}/credential` | POST | Upsert credential. Accepts JSON or form data (HTMX form submits as `application/x-www-form-urlencoded`). | Form payload includes `scope=user|company`, provider fields, `makeDefault`. | `201 Created`/`200 OK` with JSON body echoing stored record, or HTMX notice snippet plus `HX-Trigger: ai-credentials-refresh` header. |
| `/api/ai/providers/{provider}/credential/test` | POST | Validate credential without persisting. | Same payload as upsert, or `{ "credentialId": "…" }` to reuse stored secret. | Success HTMX notice (`SettingsAINoticePartial`); errors return notices with validation message. |
| `/api/ai/credentials/{credentialID}` | DELETE | Remove credential (scope permissions enforced). | `HX-Request: true` DELETE. | HTMX notice, `HX-Trigger: ai-credentials-refresh`, `204 No Content` when non-HTMX. |
| `/api/ai/providers/{provider}/events` | GET | List audit events with query filters `action`, `scope`, `actorId`. | `GET /api/ai/providers/openai/events?scope=company&limit=20` | JSON or HTMX table showing timestamp, action, actor, metadata. |
| `/api/ai/providers/{provider}/status` | GET | Render provider health badge. | `HX-Request: true` GET | Returns badge partial; failure propagates an error badge but logs details server-side. |

HTMX requests originate from the Settings page using `hx-get`, `hx-post`, and
`hx-trigger` attributes. Successful mutations add `HX-Trigger: ai-credentials-refresh`
so both the credential table and event log reload automatically.

## Settings › AI UI Behaviour
- Tabs (`SettingsShell`) display General, Users, AI based on capabilities.
- Provider sidebar: loads from catalog; switching providers triggers HTMX swap of
  the AI content root so the form/table refresh with scoped data.
- Status badge: lazy loads `status` endpoint and supports manual refresh.
- Credential form:
  - Scope radios default to `user`. “Entire company” is enabled when
    `props.CanManageCompany`; otherwise the radio is disabled with helper text.
  - Provider fields render from catalog metadata.
  - Submission targets `#ai-settings-notice` for inline success/error messages.
- Credential table:
  - Displays provider, scope label (default flagged), label, fingerprint column.
    When a suffix is stored, the column shows `...NNNN` with the original hash
    available as a tooltip for admins.
  - “Test” button calls the test endpoint; “Delete” issues HTMX delete with
    confirm prompt.
- Event log: filter form issues HTMX GET to events endpoint, updating the table in
  place.

## Security & Auditing
- Encryption: API keys encrypted with currently configured cipher before
  persistence. Cipher selection occurs during app boot (AESCipher by default).
- Resolver usage: decrypts on demand, touches credential for last-used tracking,
  and logs non-fatal errors to metrics logger.
- Metadata hygiene: `cloneMetadata` ensures we never mutate the caller’s map;
  suffix derivation trims whitespace and only stores when length ≥ 4.
- Audit trail: `recordCredentialEvent` captures create/update/delete actions with
  scope, fingerprint, label, and default flags. Event list endpoint exposes this
  data for admins to review.

## Known Limitations / Considerations
- Existing credentials must be re-saved to populate the new key suffix metadata.
- The fingerprint column still surfaces hashed data; admins needing precise
  cross-referencing must use the new suffix display. There is no bulk export.
- Catalog is static (in-memory) today; planned database-backed catalog would
  affect provider lists and field definitions (see `docs/ai-provider-catalog-spec.md`).
- No automated UI or handler tests currently assert suffix rendering or HTMX
  interactions; manual regression testing is required after UI updates.

## Potential Spec Additions
- API payload examples for provider-specific fields when new providers are added.
- End-to-end flow diagram or sequence chart once conversation/document services
  expand usage (separate task, not covered here).
- Explicit manual test checklist (currently tracked ad hoc).

