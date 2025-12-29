# ADR-003: Asynchronous Embedding Worker Queue

- **Status:** Accepted (Stage 2 implementation in flight)
- **Deciders:** Platform Engineering, Data Platform
- **Technical Story:** EGRA ingest scalability & resilience roadmap
- **Related Documents:** `docs/embedding_system_master_blueprint_v1.0.md`, `docs/ai-provider-catalog-spec.md`

## Context

Stage 1 uses a synchronous CLI to ingest documents and call OpenAI embeddings inline. This is sufficient for a single authoritative corpus, but the roadmap includes:

- Bulk ingestion of interpretive/internal documents uploaded by tenants.
- Automatic re-embedding when provider defaults change or catalog entries update.
- Retrying transient provider errors (rate limits, network issues) without blocking user workflows.

Alternatives considered:

- **Continue synchronous ingest:** Simple but risks timeouts and poor UX for larger payloads.
- **Cron-based batch jobs:** Light-weight scheduling, but offers limited observability or per-document retries.

## Decision

Adopt an asynchronous worker queue architecture in Stage 2, anchored on AWS managed services:

- Web/API tier or CLI writes paragraph metadata to Postgres and enqueues embedding jobs onto **AWS SQS** (payload: paragraph ID, content hash, provider/model hints).
- A pool of **ECS/Fargate workers** consumes jobs, resolves secrets (via AWS Secrets Manager), calls the embedding provider, and persists vectors back to Postgres/pgvector.
- Workers apply **3 attempts with exponential backoff and jitter** before pushing the job to a **dead-letter queue** for manual remediation.
- Metrics (queue depth, success/error counts, OpenAI latency) are exported to CloudWatch/Prometheus; job status is surfaced to callers via a status column or API.

Future flexibility: if we introduce Redis for low-latency caching or run in non-AWS environments, the queue interface allows swapping SQS for Redis/Asynq while keeping producer/consumer contracts unchanged.

## Consequences

- ✅ **Scalability:** Ingest returns quickly; embedding throughput scales with the number of Fargate workers.
- ✅ **Resilience:** Structured retries/backoff mitigate transient provider errors; the DLQ preserves failing jobs for audit/replay.
- ✅ **Extensibility:** Queue abstraction supports additional providers, re-embedding workflows, and eventual Redis-based deployments without changing producer logic.
- ⚠️ **Operational overhead:** Requires Terraform/CloudFormation for SQS + ECS, monitoring dashboards, and on-call readiness.
- ⚠️ **Implementation effort:** Stage 2 must ship job schema definitions, worker images, deployment automation, and job status reporting.
