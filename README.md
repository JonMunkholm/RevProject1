# RevProject1

## Monitoring

The AI credential subsystem now exposes Prometheus counters you can scrape from the application process:

- `ai_credentials_missing_total` – increments when no credential is available for a company/provider scope.
- `ai_credential_test_failures_total` – increments when a credential validation request fails.
- `ai_credential_resolve_failures_total` – increments when a credential lookup returns an error.

Ensure your Prometheus configuration picks up the application metrics endpoint after deploying these changes.

## AI Settings & Credentials

- A new `company_user_roles` table tracks workspace roles (`admin`, `member`, `viewer`). The first user in a company is seeded as `admin`; subsequent users default to `member`.
- All `/app/settings` routes now require an authenticated session. Tabs are rendered based on the requester’s capabilities (e.g., only admins can see the Users tab).
- The AI tab consumes the provider catalog directly from the backend. Provider metadata (fields, docs, models) is defined in `internal/ai/provider/catalog`.
- Provider credential endpoints are provider-scoped (`/api/ai/providers/{providerID}/...`). The UI uses HTMX to load/save/test credentials and renders inline notices/status badges based on server responses.
- Users without permission receive inline warnings rather than hidden errors; HTMX partials (`SettingsAINoticePartial`, `SettingsAIStatusBadgePartial`) are emitted by handlers when needed.

## Chat Interface (Alpha)

- Navigate to `/app/chat` to start a conversation using the currently selected provider. The UI reuses stored credentials (user → company → global) and will block message input if no key is available.
- Switching providers spins up a new conversation session; each session persists in Postgres so history can be resumed later.
- Requests rely on the existing `/api/ai/conversations` flow. Streaming responses are not yet enabled; replies render after completion. Rate-limit and credential failures return inline notices.

## Development Notes

- Generate templates after editing files under `app/pages/*.templ`:

  ```sh
  templ generate
  ```

- Run tests with a temporary Go build cache if your environment is sandboxed:

  ```sh
  GOCACHE=$(mktemp -d) go test ./...
  ```

- Apply new migrations before running the application:

  ```sh
  goose -dir sql/schema up
  ```
