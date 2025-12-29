# Stage 2 — Production Readiness Plan

> Snapshot of agreed directions before drafting ADRs or implementation tickets.

---

## Reliability Defaults
- **Async embedding pipeline**: adopt AWS SQS-backed job queue with ECS/Fargate workers (ADR-003 follow-up). Jobs include paragraph ID + hash; workers handle OpenAI calls, retries (3× exponential backoff), and status updates.  
  - *Future watch*: if we introduce Redis for other workloads or move off AWS, evaluate migrating queue + cache to Redis/Asynq.
- **Async queue implementation detail**  
  - *Queue payload*: `{ job_id, paragraph_id, source_hash, model, priority, attempt, metadata_version }`.  
  - *DB table `embedding_jobs`*: tracks status (`pending|in_progress|succeeded|failed|dead_letter`), attempts, timestamps, last_error, priority, metadata version; indexed by status/paragraph.  
  - *Infra (Terraform)*: create AWS SQS queue + DLQ (3 retries via redrive), ECS/Fargate worker service with autoscaling, IAM task role for SQS + Secrets Manager, CloudWatch logs.  
  - *Config*: env vars `EMBED_QUEUE_URL`, `EMBED_DLQ_URL`, secrets from AWS Secrets Manager; LocalStack scripts for dev.  
  - *Worker behaviour*: exponential backoff (2s base, x2 multiplier, max 30s, ±25% jitter); visibility timeout ≈ processing time; send to DLQ after max attempts while marking DB row `dead_letter`.  
  - *Status API*: `GET /api/embedding-jobs/{job_id}` returns job metadata (status, attempts, timestamps, last_error); optional list endpoint for ops.  
  - *Testing*: unit tests for producer/worker paths, integration tests using LocalStack SQS + mocked OpenAI, staging load test validating autoscaling and SLA.  
  - *Rollout*: feature flag to keep sync ingest fallback; deploy schema + producer changes first, run workers in shadow mode, then cut over once metrics stable.
- **Connection pools**: API `MaxOpenConns ≈ 2×CPU`, `MaxIdleConns ≈ CPU`, `ConnMaxLifetime ≈ 15m`, workers slightly smaller; wrap DB calls in context timeouts and expose readiness/liveness probes.
- **External API resilience**: classify retryable errors, apply backoff + circuit breaker, requeue failed jobs, emit OpenAI latency/error metrics, and log OpenAI request IDs.

## Temporal & Authority Guardrails
- **Default filters**: enforce `effective_date <= now()` and `superseded = false` for authoritative queries.
- **Role/tenant handling**: gate interpretive/internal toggles behind auth roles; honor `tenant_id` + `visibility_scope` and prepare for row-level security.
- **Data maintenance**: define accounting-owned workflow (admin UI or controlled SQL) to update effective/superseded flags; capture change audit trail.
- **Implementation detail**  
  - *Auth & claims*: introduce middleware that validates JWT/OIDC tokens, extracts `roles[]`, `tenant_id`, and `scopes`. Map roles (`accounting_admin`, `tenant_admin`, `viewer`) to feature flags (e.g., `can_include_interpretive`).  
  - *SQL changes*: update retrieval queries to inject default WHERE clause (`source_type='authoritative' AND superseded=FALSE AND effective_date <= :as_of`). Optional params (`include_superseded`, `include_interpretive`, `include_internal`) only honoured when role flag set; authority score weighting keeps authoritative results first.  
  - *RLS policies*: enable Postgres row-level security on `asc_paragraphs` and `asc_embeddings`, ensuring tenant documents (`tenant_id IS NOT NULL`) are selectable only for matching tenant or shared visibility scope; service accounts use session variables (e.g., `SET app.tenant_id`).  
  - *Metadata tooling*: add admin endpoint or CLI (`cmd/tools guidance-update`) that records changes to `effective_date`, `superseded`, `visibility_scope`, writing to `guidance_audit` table with actor, before/after values.  
  - *Testing*: unit tests for role-to-flag mapping, SQL builder permutations; integration tests ensuring unauthorized roles cannot access interpretive/internal content; regression tests for as-of queries.  
  - *Rollout*: deploy middleware + SQL behind feature flag, seed role mappings, run parallel queries to compare results, then enforce filters once validation complete.

## Retrieval Quality
- **Embedding cache**: store query hash → embedding in Redis (or equivalent) with TTL + invalidation on content updates.  
  - *Future item*: when LLM summarization is added, extend cache to response summaries.
- **Hybrid scoring**: combine pgvector cosine with Postgres full-text/BM25 (`score = α·cosine + β·bm25`), tune weights via evaluation; ensure indexes.
- **Thresholding/rerank**: enforce minimum cosine score (e.g., 0.40). Monitor precision; introduce secondary reranker if false positives rise.
- **Implementation detail**  
  - *Redis provisioning*: Terraform module `infra/redis` (Elasticache or managed alternative) with small cluster in staging, auto-failover in prod; IAM security groups limiting access to app/worker subnets.  
  - *Cache interface*: new `internal/cache` package providing `GetEmbedding(queryKey) ([]float32, bool)` / `SetEmbedding(queryKey, vector, ttl)`; query key = SHA256 of normalized query + filters + tenant/role signature.  
  - *Invalidation hooks*: on paragraph insert/update or metadata change (from guardrail workflow), publish event to clear relevant cache keys (Redis key pattern or Pub/Sub channel).  
  - *Hybrid SQL*: extend retrieval query to compute `bm25` via `ts_rank_cd` on a materialized `tsvector` column (`ALTER TABLE asc_paragraphs ADD COLUMN search_vector tsvector`, maintained by trigger). Composite score computed in SQL and ordered descending; expose α/β from config table `retrieval_weights`.  
  - *Threshold config*: store minimum cosine score in config (`retrieval_thresholds` table or env var). API returns `no_results` when best score below threshold; optionally include an explanatory message.  
  - *Rerank scaffold*: feature-flagged step that, when enabled, sends top N results to secondary evaluator (initially stubbed, later cross-encoder); log metrics comparing pre/post rerank for tuning.  
  - *Testing*: unit tests for cache normalization, SQL weighting, threshold behaviour; integration test with seeded data to validate ranking; benchmarking script measuring latency with and without cache.  
  - *Rollout*: enable cache in staging, monitor hit/miss and latency; deploy hybrid scoring behind flag, tune α/β using evaluation set, then activate in prod; keep rerank disabled until metrics justify.

## Observability & Audit
- **Logging**: structured JSON with request IDs, actor/tenant, OpenAI request IDs, DB timings, status.
- **Metrics/tracing**: instrument API + workers via OpenTelemetry/Prometheus (HTTP latency, queue depth, OpenAI errors).
- **Audit storage**: persist each search in `retrieval_log` (request ID, filters, embedding hash, result hashes, duration, error). Set retention + access controls.
- **Implementation detail**  
  - *Logging stack*: adopt zap/logrus JSON formatter; include `request_id`, `job_id`, `actor`, `tenant_id`, `openai_request_id`, `duration_ms`, `status`. Route application logs to CloudWatch (ECS) and optionally ship to ELK/Splunk.  
  - *Tracing*: integrate OpenTelemetry SDK; propagate trace/span IDs through HTTP handlers, worker jobs, and OpenAI client. Export traces to AWS X-Ray or OTLP collector (Grafana Tempo).  
  - *Metrics*: expose Prometheus endpoints (or CloudWatch embedded metrics) for API latency histograms, queue depth (`sqs_approx_number_of_messages`), worker success/failure counters, cache hit/miss rates, DB query timings.  
    - *Dev note*: local integration runs bind worker metrics to `127.0.0.1:0`, logging the assigned port. Persisting that value (e.g., temp file for scrapers) is deferred work once automated scraping is needed.
  - *retrieval_log schema*: add columns `request_id UUID`, `tenant_id TEXT`, `role TEXT`, `embedding_sha256 TEXT`, `top_results JSONB` (hashed references), `duration_ms INT`, `error TEXT`, `threshold NUMERIC`. Create partial indexes on `(ts DESC)`, `(tenant_id, ts)`.  
  - *Retention & access*: schedule partitioning/archival (e.g., monthly partitions, move older than 3 years to Glacier/S3). Restrict read access to compliance/ops roles; mask sensitive fields if required.  
  - *Alerting*: define CloudWatch/Prometheus alerts for high error rates, DLQ growth, cache miss spikes, unusual latency. Wire alerts to Slack/PagerDuty once on-call is established.  
  - *Testing*: add tests validating log fields/population, ensure audit entries written per request (integration), and verify tracing headers propagate end-to-end (API ↔ worker).  
  - *Rollout*: enable structured logging first, then tracing/metrics with dashboards; backfill `retrieval_log` schema via migration, replay sample queries to confirm entries, finally enable alerting thresholds.

## Security & Compliance
- **Secrets management**: defer until pre-launch—plan to move secrets into AWS Secrets Manager (or equivalent) and inject at runtime.
- **Access controls**: enforce JWT/OIDC auth, least-privilege DB roles, optional row-level security for tenant isolation.
- **Network hardening**: terminate TLS at ALB/API Gateway, enable WAF/rate limiting, run Postgres in private subnets, restrict SGs to app/worker traffic.
- **Compliance logging**: store hashed/limited result payloads, define retention (e.g., 3 years), encrypt logs at rest, integrate with SIEM if required.
- **Implementation detail**  
  - *Secrets rollout*: create Secrets Manager entries for `DB_URL`, `OPENAI_API_KEY`, `OPENAI_PROJECT_ID`, `JWT_SECRET`, rotated on 90-day schedule. Update ECS task definitions to fetch secrets via task role; local dev continues to use `.env` with clear documentation on differences. Plan migration checklist before go-live.  
  - *Auth enforcement*: integrate OIDC (e.g., Auth0/AWS Cognito); middleware validates tokens, populates roles/tenant claims for guardrail logic. Define DB roles (`app_read`, `app_write`, `worker_write`) with least privilege; use IAM auth for Postgres if available.  
  - *Row-level security*: enable RLS on tenant tables with policies referencing session variables (`SET app.tenant_id`); ensure service accounts set the context on connection checkout.  
  - *Network controls*: configure ALB with ACM-managed TLS cert, enforce HTTPS-only, attach AWS WAF rules (rate limit, bot filtering). Place Postgres/Redis in private subnets; security groups allow ingress from app/worker tasks only. Consider VPC endpoints for SQS/Secrets Manager.  
  - *Compliance logging*: redact or hash sensitive fields (e.g., store SHA256 of content snippets). Define retention policy (e.g., 36 months active, 84-month cold storage) and automated archival to S3 Glacier with encryption. Integrate with org SIEM (CloudWatch Logs subscription, Kinesis Firehose, etc.).  
  - *Documentation & training*: update runbooks with incident-response steps, access request process, and secret rotation calendar. Provide guidance to accounting stewards on metadata updates with audit logging.  
  - *Verification*: run security scans (dependency, container scanning), pen test ahead of launch, and validate WAF/rate limits via synthetic attacks.  
  - *Rollout*: begin with secrets manager integration in staging, then production; apply network hardening changes post-traffic rehearsal; schedule compliance logging migration concurrently with observability rollout.

## Operational Runbooks
- **Environment**: containerize services; deploy via ECS/Fargate + Terraform (managed Postgres, private networking). Document provisioning steps.
- **CI/CD (GitHub Actions)**: pipeline runs gofmt/vet/tests, migration status check, build + push image, deploy to staging, smoke test, manual approval, then prod deploy.  
  - *Future item*: migrate pipeline to Harness.io once backend stabilizes.
- **Backups/restore**: nightly encrypted `pg_dump` + WAL to S3 (SSE-S3/SSE-KMS). Maintain IAM-scoped roles, quarterly restore drills, document pgvector rebuild/requeue flow.
- **On-call procedures**: defer until user/SLA demand increases; note triggers—external customers onboarded, monitoring alerts in place, SLA commitments.

## Deferred / Future Work
- Response-summary caching once LLM output is added.
- Harness.io pipeline migration.
- Secrets manager rollout before production launch.
- Formal on-call program post external launch.

---

## Stage 2 Execution Roadmap (Draft)

1. **Foundation & Infra Setup**
   - Provision SQS/DLQ, ECS worker skeleton, Redis cluster (staging), baseline Prometheus/OTEL collector.
   - Apply Terraform modules in sandbox; verify IAM roles and VPC wiring.

2. **Async Embedding Pipeline**
   - Schema migrations (`embedding_jobs`), producer refactor, worker service implementation with retry/DLQ.
   - LocalStack integration tests, staging shadow run, production cutover behind feature flag.

3. **Temporal & Authority Guardrails**
   - Auth middleware with role/tenant claims, SQL filter updates, RLS policies.
   - Metadata tooling + audit table; staged rollout with comparison logging.

4. **Retrieval Quality Enhancements**
   - Redis cache integration, hybrid SQL scoring, threshold gating, telemetry dashboards.
   - Weight tuning via evaluation set; enable in staging then prod.

5. **Observability & Compliance Logging**
   - Structured logging rollout, OTEL tracing, Prometheus metrics, retrieval_log schema migration.
   - Alert rules + dashboards; retention/archival automation.

6. **Security Hardening**
   - Secrets Manager integration, TLS/WAF enforcement, DB role/RLS verification, compliance documentation.
   - Pen tests / security scans pre-launch.

7. **Pre-Launch Review**
   - Response-summary cache decision, on-call playbook draft, CI/CD audit (prep for Harness migration), launch readiness checklist.

---

## Backlog Ticket Drafts

Use these as epics/tasks when populating your tracker (dependencies reflect recommended sequencing).

1. **EPIC: Foundation & Infra Setup**
   - ✅ T1: Terraform module for SQS/DLQ (includes IAM policies, outputs).  
   - ✅ T2: Terraform module for ECS/Fargate worker service (task definition, autoscaling).  
   - ✅ T3: Provision Redis (Elasticache) dev/staging clusters.  
   - ✅ T4: Stand up Prometheus/OTEL collector in staging.  
     *Deployment note*: Before go-live, run `terraform apply` (or equivalent) for `infra/embedding_queue`, `infra/embedding_worker`, `infra/redis`, and deploy the Docker Compose stack in `infra/prometheus` so the async pipeline infrastructure and observability are live.
   - Dependencies: none (baseline infrastructure).

### Ticket Drafts — Foundation & Infra Setup

**T1. Terraform: Embedding Job Queue** *(Completed)*  
- *Description*: Create Terraform module that provisions `embedding_jobs` SQS queue, DLQ, redrive policy (`maxReceiveCount=3`), queue attributes (visibility timeout 30 s, retention 4 days), and IAM policies granting producer/worker permissions. Export queue URLs/ARNs via outputs + SSM params.  
- *Acceptance Criteria*:  
  1. `terraform apply` in sandbox creates queue + DLQ with correct settings.  
  2. IAM policy documents allow only required actions (`SendMessage` producer, `Receive/Delete/ChangeVisibility` worker).  
  3. Outputs documented and consumed in staging environment.  
- *Dependencies*: None.

**T2. Terraform: ECS/Fargate Worker Service** *(Completed)*  
- *Description*: Build Terraform module defining task definition, service, auto-scaling (scale up >100 msgs, down <10), CloudWatch log group, task IAM role (SQS + Secrets Manager), and security groups/subnets.  
- *Acceptance Criteria*:  
  1. Service deploys in staging with desired count configurable (default 2).  
  2. Task role can read secrets and interact with queue.  
  3. Autoscaling policy triggers in load test (document validation steps).  
- *Dependencies*: T1 (queue outputs).

**T3. Provision Redis Cluster** *(Completed)*  
- *Description*: Use Terraform (or cloud console script) to provision Elasticache Redis (dev single-node, staging/prod with replica & auto-failover), configure parameter group, subnet group, security groups, and expose connection info via SSM.  
- *Acceptance Criteria*:  
  1. Redis endpoints reachable from app/worker subnets only.  
  2. Metrics/monitoring alarms for CPU/memory/engine health configured.  
  3. Run connectivity test from staging app to verify TLS/auth (if enabled).  
- *Dependencies*: None (parallel to T1/T2).

**T4. Prometheus & OTEL Collector Setup** *(Completed)*  
- *Description*: Deploy Prometheus (or managed alternative) plus OpenTelemetry collector in staging; configure scrapings for API, worker, Redis, Postgres, SQS metrics (via CloudWatch exporter). Provide dashboard skeleton (Grafana or CloudWatch).  
- *Acceptance Criteria*:  
  1. Metrics endpoint accessible and showing API latency, queue depth, worker success/failure.  
  2. Alerts configured for key thresholds (queue backlog, error rate).  
  3. Documentation for adding new metrics/traces published.  
 - *Artifacts*: `infra/prometheus/docker-compose.yaml`, `prometheus.yml`, `otel-collector-config.yaml`, `rules/alerts.yml`, README with usage instructions.  
- *Dependencies*: Relies on T1/T2 outputs for queue metrics; can begin once staging infra exists.

2. **EPIC: Async Embedding Pipeline**
   - T1: DB migration creating `embedding_jobs` table + triggers.  
   - T2: Refactor ingest producer to enqueue jobs + status API.  
   - T3: Implement worker service with retry/DLQ handling.  
   - T4: Integration tests using LocalStack + mocked OpenAI.  
   - T5: Staging shadow run + feature-flagged production cutover.  
   - Dependencies: Foundation infra.

### Ticket Drafts — Async Embedding Pipeline

**T1. Migration & Schema Updates** *(Completed – see `sql/schema/019_embedding_jobs.sql`, `internal/stage1/schema.go`)*  
- *Description*: Add Go migrate/goose migration creating `embedding_jobs` table with columns/constraints specified, indexes, and triggers updating `updated_at`. Update ORM/SQL layer to include new statuses and paragraph embedding_state.  
- *Acceptance Criteria*:  
  1. Migration applies cleanly and is reversible (`down`).  
  2. Tests confirm table defaults (`status='pending'`, `attempts=0`).  
  3. Paragraph records reflect embedding state transitions.  
- *Dependencies*: Foundation infra not strictly needed; run after T1 of Foundation if referencing queue outputs.

**T2. Producer Refactor & Status API** *(In Progress – ingest CLI enqueues, added `/api/embedding-jobs/{id}` status endpoint)*  
- *Description*: Modify ingest CLI/API to insert paragraph + job row, publish message to SQS, and expose `GET /api/embedding-jobs/{job_id}` endpoint returning status. Ensure back-compat flag for synchronous path.  
- *Acceptance Criteria*:  
  1. Publishing to queue succeeds with retry on transient errors.  
  2. Status endpoint returns `pending` immediately after enqueue and updates when worker finishes (integration test).  
  3. Feature flag allows disabling async path to fall back to sync.  
- *Dependencies*: T1, Foundation SQS module.

**T3. Worker Service Implementation** *(In Progress – polling loop, Prometheus metrics, visibility management in `cmd/worker`)*  
- *Description*: Implement Go worker (new cmd/service) consuming SQS, fetching job row, calling OpenAI, writing embedding, updating job + paragraph status, handling retries, sending to DLQ. Include structured logging and metrics.  
- *Acceptance Criteria*:  
  1. Worker processes job successfully, respecting visibility timeout and retries.  
  2. On 3 failures, job moved to DLQ and DB marked `dead_letter` with error detail.  
  3. Metrics/logs emit job status, latency, error counts.  
- *Dependencies*: T1, T2, Foundation ECS module.

**T4. Integration & Load Testing** *(Scaffold in progress – see `integration/` harness)*  
- *Description*: Build suite running LocalStack SQS + mocked OpenAI to validate end-to-end flow (producer → worker → DB). Produce load scripts and run in staging to exercise autoscaling thresholds.  
- *Acceptance Criteria*:  
  1. CI job executes LocalStack test successfully.  
  2. Staging load run demonstrates workers scaling and meeting SLA (< defined processing time).  
  3. Test documentation stored with reproduction steps.  
- *Dependencies*: T2, T3.

**T5. Shadow Run & Cutover**  
- *Description*: Deploy async pipeline in staging, shadow existing sync ingest (run both but store only from async path), monitor metrics, then enable async-only in production once stable. Document fallback procedure.  
- *Acceptance Criteria*:  
  1. Shadow run shows parity between sync & async outputs for sample dataset.  
  2. Feature flag switched to async-only in prod with no errors for 48h.  
  3. Runbook updated with rollback steps (toggle flag, drain queue, clean DLQ).  
- *Dependencies*: T4 completion.

**Deployment Notes (Env & Infra)**  
- Worker/services require `EMBED_QUEUE_URL` and `EMBED_DLQ_URL`; DLQ redrive policy should match the worker `EMBED_MAX_ATTEMPTS` (3 by default).  
- Document the AWS/Terraform change to provision the DLQ and attach the redrive policy before cutover; ensure staging/prod `.env`/Secrets Manager entries add the new vars.  
- Update the launch runbook with rollback guidance (clear DLQ, re-enable sync ingest) and reference the integration harness (`integration/setup.sh`, `integration/run.sh`) as the regression check.  
- Add these configuration steps to the go-live checklist so ops can verify queue URLs and backoff limits during deployment reviews.
- Extend the worker to publish core counters (processed, failed, dead_letter) to CloudWatch alongside Prometheus so queue/worker alarms can be enabled immediately.
- Stand up CloudWatch alarms/dashboards for queue depth, age-of-oldest message, DLQ count, and job failure rate; migrate or mirror to Prometheus/Grafana once that stack ships.
- Async disable flag now ships with `cmd/ingest --disable-async` / `ASYNC_EMBEDDING_DISABLED=true`; update runbooks so incidents flip the toggle, drain DLQ, and re-enable async when resolved. LocalStack smoke test remains the verification step.
- Track the structured logging migration under the observability epic to ensure JSON logs (zap/logrus) are in place ahead of launch.
- Record the communications/escalation plan (Slack/PD contacts) as a launch follow-up so it’s assigned before go-live.
- **CloudWatch Metric Map** (add to ops runbook/terraform outputs)
  - `Namespace`: `EGRA/EmbeddingWorker`
  - Default dimensions: `Service=embedding_worker`, `Environment=<staging|prod>`
  - Counter metrics:  
    - `JobsProcessed` (Count) — emit on successful completion.  
    - `JobsFailed` (Count, `Result=failed|dead_letter`) — emit on any failure.  
    - `JobsDeadLetter` (Count, `Result=dead_letter`) — emit when task transitions to DLQ.  
  - Latency metrics:  
    - `JobDurationSeconds` (Seconds, `Result=success|failed|dead_letter`).  
    - `OpenAIRequestSeconds` (Seconds, `Result=success|error`).  
  - Infra follow-up: ensure worker IAM role allows `cloudwatch:PutMetricData`; capture target alarm thresholds (queue depth >25 for 5m, age-of-oldest >60s, DLQ count ≥1) in Terraform.
- **Async Disable Flag & Runbook Hooks**
  - Env flag `ASYNC_EMBEDDING_DISABLED=true` (or CLI `cmd/ingest --disable-async`) forces ingest back to synchronous path; document the toggle location (Secrets Manager/feature flag service) and steps to revert once incident resolved.  
  - Runbook checklist (validated via LocalStack harness):  
    1. Flip async-disable flag.  
    2. Purge or drain DLQ (`aws sqs purge-queue/send-message-batch`).  
    3. Replay messages once backlog cleared.  
    4. Execute `integration/run.sh` (LocalStack harness) or staging smoke ingest to confirm recovery.  
  - Follow-up: confirm production Secrets Manager entries expose the flag and runbook references the CLI override for local troubleshooting.
- Latest validation (2025-10-23):  
  - `integration/run.sh` run locally → happy-path job succeeded, failure fixture moved to DLQ after 3 attempts (expected 3072 vs 1536 dimensions), worker logs and SQS attributes captured.  
  - `ASYNC_EMBEDDING_DISABLED=true go run ./cmd/ingest ...` logged “async embedding disabled; running synchronous ingest” and wrote embedding synchronously, confirming the toggle works end-to-end.
- **Deferred — Operations & Observability Backlog** (no impact on functional scope)
  - CloudWatch IAM updates, alarm provisioning, and dashboard build-out.
  - Async-disable flag documentation and DLQ replay automation.
  - Structured logging migration + SIEM integration work.
  - Communications/escalation plan ownership (Slack, PagerDuty routes).
  - Post-launch review to tune queue thresholds once real traffic data is available.
  - Staging/QA alarm threshold strategy and synthetic smoke tests.

3. **EPIC: Temporal & Authority Guardrails**
   - T1: Auth middleware + role/tenant claim mapping.  
   - T2: Update SQL queries with default filters and optional flags.  
   - T3: Enable RLS policies and session context handling.  
   - T4: Build metadata admin tooling + audit table.  
   - T5: Validation rollout (comparison logging, toggle in prod).  
   - Dependencies: Async pipeline (for shared auth context), optional redis not required.

### Ticket Drafts — Temporal & Authority Guardrails

**T1. Auth Middleware & Role Mapping**  
- *Description*: Extend existing JWT middleware to accept OIDC-access tokens (Auth0/Okta compatible), validate issuer/audience, and extract `sub`, `tenant_id`, `roles[]`, `scopes[]` into `auth.Session`. Keep the canonical role→capability map in Go for launch; document the DB/config-driven alternative as future work once ops need runtime edits.  
- *Acceptance Criteria*:  
  1. Middleware rejects invalid/expired tokens and populates expected claims in context.  
  2. Unit tests cover role mappings and missing claim scenarios.  
  3. API docs updated to describe new auth requirements.  
- *Dependencies*: Async pipeline (shared auth libs), security groundwork for OIDC provider.

**T2. Retrieval SQL Guardrails**  
- *Description*: Refactor retrieval service to apply default filters (`source_type='authoritative'`, `superseded=FALSE`, `effective_date <= :as_of`) directly in SQL, add optional clauses when authorized roles request interpretive/internal or superseded content, and surface meaningful errors when callers lack capability flags. Ensure ordering continues to bias authority scores.  
- *Acceptance Criteria*:  
  1. Queries enforce defaults even when callers omit optional parameters.  
  2. Tests demonstrate that unauthorized roles cannot retrieve interpretive/internal content.  
  3. `as_of` parameter supported for authorized roles, returning point-in-time results.  
- *Dependencies*: T1 (claims available).

**T3. Postgres Row-Level Security Policies**  
- *Description*: Enable RLS on `asc_paragraphs`/`asc_embeddings`. Define policies allowing access when `tenant_id` is NULL (public authoritative) or matches session `app.tenant_id`, or when `visibility_scope` contains `'public'`. Update DB connection helpers to set `SET app.tenant_id`, `app.roles`, and `app.capabilities` on every checkout so pooled connections respect tenant scoping.  
- *Acceptance Criteria*:  
  1. RLS policies tested (psql + integration) to confirm unauthorized access blocked.  
  2. Connection pool sets session variables on checkout; errors logged if missing.  
  3. Documentation for managing tenant visibility and debugging RLS issues.  
- *Dependencies*: T1 (tenant claims).

**T4. Metadata Governance Tooling**  
- *Description*: Implement an accounting-operated CLI (wrapping signed requests or direct DB calls) for updating `effective_date`, `superseded`, `visibility_scope`, with guardrails to prevent invalid states. Create `guidance_audit` table capturing before/after values, actor, timestamp. Provide minimal UI or script for accounting stewards.  
- *Acceptance Criteria*:  
  1. Audit table populated for every metadata change.  
  2. Tooling enforces validation (e.g., cannot clear `effective_date` for authoritative content).  
  3. Documentation/training notes for accounting team.  
- *Dependencies*: T2 (filters rely on accurate metadata).

  **Planned Implementation (2025-10-24 design sync)**  
  - Database: add `guidance_audit` (id uuid, paragraph_id uuid, change_type text, actor uuid/text, before jsonb, after jsonb, reason text, created_at timestamptz). Add helper view summarising latest state for reporting.  
  - CLI (`cmd/tools guidance`): supports commands `list-pending` (show authoritative paragraphs missing effective_date), `update --paragraph <id> --effective <date> --superseded=<bool> --reason <text>`, `visibility --paragraph <id> --scope tenant|public`. CLI requires `GUIDANCE_ACTOR` env for audit actor id and uses application DB URL.  
  - Validation: disallow clearing `effective_date`/`superseded=false` once audit entry exists unless `--force` flag provided and recorded. Guardrail ensures tenant-specific visibility requires `--tenant-id`.  
  - Ops workflow: accounting runs CLI locally (or via GitHub Action) with 4-eyes review; output audit rows for review (CSV/JSON). Runbook will document: backup paragraph row (`select * from asc_paragraphs where id = ...`), run CLI, verify guardrail logs, and attach audit diff to change request.
  - Validation/Runbook: after each CLI invocation run `select change_type, actor, reason, created_at from guidance_audit order by created_at desc limit 5;` to confirm audit trail, and `select effective_date, superseded from asc_paragraphs where id = ...;` to verify metadata. Incident rollback is a second CLI invocation with previous values and a new audit reason referencing the change request.
  - TODO (ops doc): append guidance CLI workflow + rollback checklist to the accounting runbook and circulate for approval.

**T5. Validation Rollout & Feature Flag**  
- *Description*: Deploy guardrails behind a feature flag (`GUARDRAILS_ENABLED`) with request logging for both the old and new paths. Run comparison logging (before/after results) in staging and production read-only mode, verify differences, gather approvals, then enforce guardrails globally. Implement fallback toggle.  
- *Acceptance Criteria*:  
  1. Comparison reports reviewed with stakeholders; any regressions resolved.  
  2. Feature flag enabled in prod; monitoring confirms expected behaviour (no superseded leak).  
  3. Rollback steps documented (toggle flag, revert RLS if needed).  
- *Dependencies*: T2–T4 completion.

**Implementation Notes (2025-10-23 sync)**  
- Guardrail claims will rely on OIDC/JWT tokens; `auth.Session` will own tenant/role/scopes and expose helper methods for downstream packages.  
- Capability mapping remains hard-coded for launch; if ops later need runtime edits we will revisit a DB-backed map (captured as a potential enhancement).  
- Retrieval SQL will short-circuit unauthorized requests and emit structured logs when overrides are denied.  
- Connection pool helpers must set `SET app.tenant_id`, `app.roles`, and `app.capabilities` after every checkout; failure to set context should log and deny the request.  
- Retrieval unit tests cover guardrail SQL construction (source-type array, tenant enforcement, as-of filtering) and ensure internal content requests without tenant context surface `ErrTenantRequired`; the service now sets `set_config(app.guardrails.*)` inside a read-only transaction so upcoming RLS policies can consume the same context.
- Integration script now starts the API twice (guardrails disabled/enabled) to verify baseline search succeeds; guardrail params (`includeInternal`, `asOf`) respond with 400 when guardrail permissions are missing, confirming the API rejects escalation attempts.
- Next up for rollout readiness: add optional comparison logging so the API can sample requests, execute both legacy and guardrail paths, and emit structured diffs before the flag is fully enabled. Plan is to introduce env toggles (`GUARDRAILS_COMPARE_ENABLED`, `GUARDRAILS_COMPARE_SAMPLE_RATE`) and log top result deltas plus status reasons. Should also store a short lived diff table or structured log (`guardrail_comparison` logger) for review.
- Comparison logging details:
  - Config knobs: `GUARDRAILS_COMPARE_ENABLED` (bool), `GUARDRAILS_COMPARE_SAMPLE_RATE` (0–1), and optional `GUARDRAILS_COMPARE_MAX_RESULTS` to cap the number of rows logged per sample.
  - When enabled and a sample is chosen, the retrieval service will run the guardrailed query first, then execute the legacy query using the same embedding (no guardrail filters) within the same request context.
  - We will compute the differences on paragraph IDs/ordering and emit a structured log (`{"event":"guardrail_comparison","sample":...,"guardrail_results":[...],"legacy_results":[...],"diff":{"added":[],"removed":[],"reordered":[]}}`).
  - No user-visible change—logs feed into staging analysis before flipping the production flag. Longer term, we can attach metrics or persist to a temporary table if paginated review is needed.
  - Validation plan: in staging, run with `GUARDRAILS_COMPARE_ENABLED=true` and a small sample rate (e.g., 0.1). Tail `guardrail_comparison` logs to confirm entries include both result sets and diff metadata; review for any unexpected guardrail-only matches before enabling the flag globally.
  - TODO (staging rollout): enable comparison logging in staging, capture sample logs, and summarise findings in guardrail release notes prior to production cutover.
- TODO: After RLS rolls out, extend integration smoke tests (`integration/run.sh`) to exercise guardrails on/off and capture comparison logs for feature-flag validation.
- Guidance CLI (`cmd/tools/guidance`) now implements the initial `update` flow for authoritative metadata, logging every change to `guidance_audit` with before/after JSON and operator reason.
- Metadata stewardship flows through a CLI that records every change in `guidance_audit`, keeping accounting accountable without forcing a full admin UI yet.  
- Feature flag + dual logging is mandatory for rollout; disabling the flag reverts to Stage 1 behaviour without dropping RLS (but RLS can be toggled if rollback escalates).
- API surface now accepts guardrail query parameters (`includeSuperseded`, `includeInterpretive`, `includeInternal`, `asOf`) only when `GUARDRAILS_ENABLED` is true and the caller holds the corresponding capability/scope; otherwise requests are rejected with 403/400.
- Added unit coverage in `internal/retrieval/retrieval_test.go` to ensure guardrail SQL construction (source-type array, tenant enforcement, as-of filtering) behaves as designed and that internal content requests without tenant context surface `ErrTenantRequired`.
- Environment flag `GUARDRAILS_ENABLED` is now wired into the API service; when set `false` the retrieval stack stays on Stage 1 behaviour, and when `true` the guardrail-aware SQL path (in progress) will activate.

4. **EPIC: Retrieval Quality**
   - T1: Cache interface + Redis wiring in retrieval service.  
   - T2: `tsvector` column, triggers, and hybrid SQL scoring with config weights.  
   - T3: Threshold gating + user-facing messaging.  
   - T4: Evaluation harness to tune α/β and threshold.  
   - T5: Optional rerank scaffolding behind feature flag.  
   - Dependencies: Redis from Foundation, guardrail SQL updates.

### Ticket Drafts — Retrieval Quality

**T1. Redis Cache Integration**  
- *Description*: Implement `internal/cache` package, inject Redis client into retrieval service, cache embeddings using normalized query key (hash of query + filters + tenant/role). Add TTL configuration and invalidation hooks responding to paragraph/metadata changes.  
- *Acceptance Criteria*:  
  1. Cache hit prevents OpenAI call and reduces latency in tests.  
  2. Cache invalidation triggered when content or metadata impacting results changes.  
  3. Metrics/logs report hit/miss rates.  
- *Dependencies*: Foundation Redis, guardrail metadata triggers.

**T2. Hybrid SQL & Indexing**  
- *Description*: Add `search_vector tsvector` column on `asc_paragraphs`, populate via trigger (covering reference, content, keywords). Update retrieval SQL to compute BM25 (`ts_rank_cd`) and combine with cosine using configurable weights stored in `retrieval_weights` table. Create necessary GIN indexes.  
- *Acceptance Criteria*:  
  1. Migration adds column/trigger/index without full table rewrite (use concurrent index).  
  2. Composite score calculation configurable without redeploy.  
  3. Tests show improved ranking for citation queries.  
- *Dependencies*: T1 optional, guardrail SQL.

**T3. Threshold Enforcement**  
- *Description*: Introduce configurable minimum cosine score; if the best result is below threshold, API returns empty results with message (`"No authoritative content meets confidence threshold"`). Allow authorized roles to override threshold via param if needed.  
- *Acceptance Criteria*:  
  1. Threshold default (0.40) applied in staging/prod; tests cover boundary cases.  
  2. Audit log records threshold used per request.  
  3. Config change (DB or env) updates threshold without restart.  
- *Dependencies*: T2 (score calculation).

**T4. Evaluation & Tuning Harness**  
- *Description*: Build offline evaluation scripts using labelled queries to tune α/β weights and threshold. Integrate with CI or manual job, output precision/recall metrics, store results for review.  
- *Acceptance Criteria*:  
  1. Harness runs against staging snapshot and produces metrics report.  
  2. Chosen weights/threshold documented with rationale.  
  3. Regression alert when future changes degrade metrics beyond tolerance.  
- *Dependencies*: T1–T3.

**T5. Rerank Feature Flag**  
- *Description*: Create optional rerank pipeline for top N results (placeholder cross-encoder or rule-based priority). Feature-flagged off by default; logs comparison metrics when enabled in staging.  
- *Acceptance Criteria*:  
  1. Rerank path can be toggled via config with no downtime.  
  2. Metrics compare base vs rerank scores; decision documented whether to enable.  
  3. No regression when rerank disabled (fast path unaffected).  
- *Dependencies*: T2, T4.

5. **EPIC: Observability & Compliance**
   - T1: Structured logging upgrade + log aggregation routing.  
   - T2: OTEL tracing integration end-to-end.  
   - T3: Prometheus dashboards & alert rules (API, queue, cache, DB).  
   - T4: `retrieval_log` schema migration + partitioning/retention automation.  
   - T5: Compliance archival pipeline (S3/Glacier) & SIEM feed.  
   - Dependencies: Async pipeline + guardrails instrumentation points.

### Ticket Drafts — Observability & Compliance

**T1. Structured Logging Rollout**  
- *Description*: Replace existing log calls with structured logger (zap/logrus), ensure consistent fields (`timestamp`, `level`, `request_id`, `actor`, `tenant_id`, `job_id`). Configure ECS tasks to ship logs to CloudWatch/Splunk.  
- *Acceptance Criteria*:  
  1. All services emit structured JSON logs; legacy printf removed.  
  2. Log aggregation pipeline verified (logs searchable by request/job ID).  
  3. Documentation for adding new log fields provided.  
- *Dependencies*: None (but coordinate with security for PII considerations).

**T2. OpenTelemetry Tracing**  
- *Description*: Instrument API handlers, ingestion pipeline, worker, and DB/OpenAI clients with OTEL spans. Configure exporter (X-Ray/OTLP) and ensure trace context propagates via HTTP headers/SQS message attributes.  
- *Acceptance Criteria*:  
  1. End-to-end trace visible for sample request (API → queue → worker → DB).  
  2. Trace sampling configurable per environment.  
  3. Errors surfaced in traces with relevant attributes.  
- *Dependencies*: T1 optional, foundation OTEL collector.

**T3. Metrics & Dashboards**  
- *Description*: Define Prometheus metrics (histograms/counters) for API latency, queue depth, worker throughput, cache hit/miss, DB query timing. Create Grafana dashboards and alert rules (e.g., queue backlog, error rate, low cache hit).  
- *Acceptance Criteria*:  
  1. Dashboards deployed and reviewed with stakeholders.  
  2. Alerts fire in staging when thresholds breached (test-run).  
  3. Runbook entries include troubleshooting steps referencing dashboards.  
- *Dependencies*: T1/T2 instrumentation.

**T4. Retrieval Log Schema & Retention**  
- *Description*: Apply migration adding columns (request_id, tenant_id, role, embedding hash, top result hashes, duration, threshold) with partitioning strategy (monthly). Implement archival job moving partitions older than 36 months to S3 with encryption.  
- *Acceptance Criteria*:  
  1. Migration executes without locking critical tables (use concurrent operations).  
  2. Audit log entries include new fields; integration test verifies write path.  
  3. Archival job runs on schedule and records success/failure.  
- *Dependencies*: T3 (for metrics/alerts around archival job).

**T5. Compliance Pipeline & SIEM Feed**  
- *Description*: Stream structured logs/audits to SIEM (CloudWatch subscription → Kinesis Firehose → Splunk/Elastic). Configure retention policies, access controls, and compliance reporting queries.  
- *Acceptance Criteria*:  
  1. SIEM receives data within acceptable latency and indexes expected fields.  
  2. Access controls enforced (only compliance team).  
  3. Reporting queries produce compliance evidence (e.g., for “show all queries by tenant X in last 30 days”).  
- *Dependencies*: T1–T4.

6. **EPIC: Security Hardening**
   - T1: Secrets Manager integration (staging → prod).  
   - T2: TLS/WAF enforcement and VPC security group tightening.  
   - T3: DB roles/RLS validation & IAM auth experiment.  
   - T4: Compliance documentation + security scans / pen test.  
   - Dependencies: Observability (for monitoring changes), guardrails (RLS).

### Ticket Drafts — Security Hardening

**T1. Secrets Manager Integration**  
- *Description*: Store production secrets in AWS Secrets Manager, update ECS task definitions to reference secrets via task role, adjust config loading to prefer Secrets Manager over `.env`. Roll out staging first, then prod.  
- *Acceptance Criteria*:  
  1. Services start successfully pulling secrets from Secrets Manager (validated via logs).  
  2. Secrets rotation playbook documented (manual rotation test executed).  
  3. `.env` usage restricted to local dev; documentation updated.  
- *Dependencies*: Observability metrics to monitor errors post-rollout.

**T2. TLS/WAF & Network Controls**  
- *Description*: Configure ALB with ACM certificate, enforce HTTPS-only, attach AWS WAF rules, tighten security groups, ensure Postgres/Redis in private subnets, add VPC endpoints for SQS/Secrets where applicable.  
- *Acceptance Criteria*:  
  1. External endpoints only accessible via HTTPS; http requests redirected/blocked.  
  2. WAF logging shows rule hits; rate limits tested.  
  3. No unintended service disruption (connectivity tests pass).  
- *Dependencies*: T1 (update configs), observability to monitor.

**T3. DB Roles & IAM Auth**  
- *Description*: Define least-privilege roles (`app_read`, `app_write`, `worker_write`), apply to services; evaluate IAM-based auth for Postgres (optional). Validate RLS policies with new roles.  
- *Acceptance Criteria*:  
  1. Role assignments updated; superuser credentials removed from app config.  
  2. RLS constraints verified for each role via tests.  
  3. Decision recorded on IAM auth feasibility.  
- *Dependencies*: Guardrails RLS, T1.

**T4. Security Testing & Documentation**  
- *Description*: Run dependency scanning, container scanning, and coordinate pen test or security review. Update compliance/security documentation (runbooks, data flow diagrams, incident response).  
- *Acceptance Criteria*:  
  1. Scan reports triaged; high/critical issues remediated.  
  2. Pen test findings addressed or accepted with mitigation plan.  
  3. Documentation updated and shared with stakeholders.  
- *Dependencies*: Prior security improvements largely in place.

7. **EPIC: Launch Readiness**
   - T1: Response-summary caching go/no-go decision.  
   - T2: Draft on-call playbook + alert tuning.  
   - T3: CI/CD audit + Harness migration planning ticket.  
   - T4: Final launch checklist & stakeholder signoff.  
   - Dependencies: Completion of prior epics.

### Ticket Drafts — Launch Readiness

**T1. Response Summary Cache Decision**  
- *Description*: Evaluate whether to implement response-summary caching (LLM output). Document pros/cons, dependencies (LLM adoption timeline), and, if deferring, record conditions for future revisit.  
- *Acceptance Criteria*:  
  1. Decision document approved by stakeholders.  
  2. Follow-up tasks created if proceeding; otherwise backlog item updated with revisit trigger.  
  3. Recorded in decision log/ADR if needed.  
- *Dependencies*: Retrieval quality metrics.

**T2. On-call Playbook & Alert Review**  
- *Description*: Draft on-call runbook covering alert triggers, diagnostic steps, escalation. Review alert thresholds from observability epic. Conduct tabletop incident drill.  
- *Acceptance Criteria*:  
  1. Runbook published and reviewed by ops/stakeholders.  
  2. Alert thresholds tuned based on drill feedback.  
  3. On-call rotation / schedule proposed (even if deferred until launch).  
- *Dependencies*: Observability alerts in place.

**T3. CI/CD Audit & Harness Plan**  
- *Description*: Audit GitHub Actions pipeline for coverage (lint, tests, security scans, infra). Draft migration plan to Harness (scope, prerequisites, timeline).  
- *Acceptance Criteria*:  
  1. Audit checklist completed; gaps tracked as tickets.  
  2. Harness migration RFC prepared with estimated effort.  
  3. Pipeline documentation updated (current + target state).  
- *Dependencies*: Earlier epics to ensure pipeline includes new checks.

**T4. Launch Checklist & Signoff**  
- *Description*: Compile final launch checklist (infra readiness, security signoff, documentation, backups, on-call readiness). Schedule go/no-go review with stakeholders and log signoff.  
- *Acceptance Criteria*:  
  1. Checklist completed with evidence links.  
  2. Sign-off meeting completed; decision recorded.  
  3. Post-launch monitoring plan agreed.  
- *Dependencies*: Completion of prior tickets.
