# Prometheus & OpenTelemetry Collector (Staging)

Containerised observability stack for staging. The `docker-compose.yaml` file
starts Prometheus and an OpenTelemetry Collector. Configuration files live in
this directory so they can be mounted into the containers.

## Prerequisites

- Docker / Docker Compose v2
- Application services expose metrics/traces (e.g., `/metrics` on `api:2112` and
  worker at `worker:2112`, OTLP exporters pointing at `localhost:4317/4318`).

## Usage

```bash
cd infra/prometheus
docker compose up -d
```

Prometheus will be available at <http://localhost:9090>. The OTEL collector
accepts OTLP gRPC on `localhost:4317` and HTTP on `localhost:4318`.

## Files

- `docker-compose.yaml` — runs Prometheus + OTEL collector.
- `prometheus.yml` — scrape config with placeholders for API/worker metrics (loads alerts in `rules/`).
- `otel-collector-config.yaml` — receives OTLP traffic and forwards to Prometheus.
- `rules/alerts.yml` — starter alert rules (queue backlog, worker failures, API latency).

Adjust targets/ports as required for the staging environment before deploying.
