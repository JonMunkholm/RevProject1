# RevProject1

## Monitoring

The AI credential subsystem now exposes Prometheus counters you can scrape from the application process:

- `ai_credentials_missing_total` – increments when no credential is available for a company/provider scope.
- `ai_credential_test_failures_total` – increments when a credential validation request fails.
- `ai_credential_resolve_failures_total` – increments when a credential lookup returns an error.

Ensure your Prometheus configuration picks up the application metrics endpoint after deploying these changes.
