# AI Endpoint Integration Roadmap

## Outstanding Decisions / Follow-up

1. **Tool roster** — Decide which endpoints receive dedicated tool wrappers versus living in the generic catalog; assign owners and define testing expectations for each tool.
2. **Endpoint catalog governance** — Choose the catalog format (file vs. DB), review procedure, and automation to keep entries in sync with actual routes.
3. **Generic router policy** — Finalize the whitelist of safe endpoints/methods, rate limits, payload caps, and any response redaction/truncation rules.
4. **Deterministic reports** — Identify heavy datasets (e.g., full customer export), determine generation schedule/delivery format, and how the assistant references them.
5. **Auth & auditing** — Confirm JWT/tenant propagation for tools and router calls; define logging, redaction, and audit trails.
6. **Prompt & UI updates** — Lock in system prompt guidance, structured error messaging, and any UI affordances (confirmation flows, download links).
7. **Testing & monitoring** — Outline unit/integration test coverage, plus monitoring/metrics instrumentation for the new tools and router usage.
8. **Product decisions** — Set expectations for latency, error handling, and whether human confirmation is required for sensitive operations.

These items should be addressed before expanding the assistant with the endpoint catalog + generic router architecture.
