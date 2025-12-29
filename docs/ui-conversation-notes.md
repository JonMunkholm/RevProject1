# Conversation UI – Design Notes

## Goals
- Allow authenticated users to initiate and continue LLM conversations using the provider/credential cascade (user → company → global defaults).
- Reuse existing conversation persistence so chat history can be resumed, audited, or reset.
- Keep UI implementation aligned with current stack (templ + HTMX, go handlers).

## Session Model
- Reuse `/api/ai/conversations` endpoints (persistent `ai_conversations` + `ai_conversation_messages` tables).
- UI to:
  - Start new conversation (`POST /api/ai/conversations`).
  - List existing sessions (optional Phase 2).
  - Reset/Delete conversation (optional Phase 2 but ensure backend supports clean-up).

## Provider Selection & Scoping
- Default provider cascade: user default → company default → global default (current resolver behaviour).
- UI behaviour:
  - Show active provider (label from catalog).
  - Allow switching provider via dropdown; switching creates a new conversation seeded with the selected provider.
  - Indicate provider source (personal/company/global) once UI reads metadata.

## Credential Fallback
- If no credential resolves:
  - Block message input.
  - Show guidance: “Add provider key in Settings → AI”.
  - Optionally link to settings route.
- Do not fall back to any hard-coded keys for security.

## Streaming vs Full Response
- Phase 1: full response (existing providers return full payload).
- Optional Phase 2: investigate streaming (chunked responses) if latency becomes an issue.

## Rate Limits & Error UX
- Backend:
  - Add single retry/backoff for transient 429/5xx responses.
  - Log structured errors with provider ID, status, latency.
  - Return canonical status codes to UI (429, 401, 500).
- UI:
  - Disable send button while awaiting response.
  - Show spinner / “Thinking…” indicator.
  - Surface friendly notices on failure (quota exceeded, invalid key, etc.).

## Multi-user Visibility
- Phase 1: personal conversations (saved per user). No share controls.
- Phase 2: extend schema with visibility flag (private/shared) and permissions to show team conversations.

## UI Structure (HTMX + templ)
- `app/pages/chat.templ` (new):
  - Provider selector (HTMX GET to reload conversation when selection changes).
  - Transcript container (`hx-target` updated when new messages arrive).
  - Message form (`hx-post` to append, disables submit while pending).
  - “Start new conversation” button (POST to create session, resets transcript).
- Layout:
  - Add `/app/chat` route to render template.
  - Reuse `SettingsShell` style or create a distinct chat layout (decide UI styling).

## Backend Notes
- `internal/application/routes.go`:
  - Add `/app/chat` GET -> render chat page.
  - Add `/api/chat/conversations` (wrap existing conversation handlers; or alias existing ones).
- `internal/handler/ai.go` already has conversation endpoints; ensure they return partials if HTMX expects them (e.g., transcript partial).
- Consider adding transcript partial template for message list.

## Testing
- Integration tests (Go) to cover:
  - Conversation creation default provider resolution.
  - Provider switch resets conversation.
  - Error propagation when credentials missing (expect 4xx).
- HTMX load tests manual (for now) since templ partials will need manual verification.

## Docs/Runbooks
- Update README or new doc detailing how to use the chat UI.
- Mention credential requirements and error messages.
- Add deferred backlog for shared conversations and streaming support.

## Follow-ups
- Add SRI hash for the `json-enc` extension or host the script locally alongside the existing HTMX asset.
- Emit structured logging/metrics around `decodeJSON` failures to surface payload regressions quickly.
- Add smoke coverage for `/app/chat` HTMX flows (create conversation, append message) once test scaffolding is ready.
- When the provider/key dropdown ships, wire conversation filtering to the selected credential so the history reflects the active scope.
- Persist short previews (first message, last assistant reply) into conversation metadata so sidebar snippets stay accurate instead of falling back to titles.
- Provide an accessible “Load more conversations” control alongside the infinite-scroll sentinel for keyboard-only users.
- Define and implement chat file-attachment support (storage backend, size/type limits, scanning, and UI affordance) once requirements are finalized.
- 2025-10-24: Conversations are now created lazily on first message send; the chat form posts the active provider so empty sessions are no longer inserted. To clean up earlier empty shells, run `DELETE FROM ai_conversation_sessions s WHERE NOT EXISTS (SELECT 1 FROM ai_conversation_messages m WHERE m.session_id = s.id);`.
- 2025-10-24: Composer form now uses a standard `hx-trigger="submit"` so HTMX posts the message payload correctly (previous `submit from:#chat-input` never fired).
- 2025-10-24: Added `list_customers` and expanded `fetch_customer` tool wiring; follow-up to support email-based lookups once customer account metadata is captured.
