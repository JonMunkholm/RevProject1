#!/usr/bin/env bash
set -euo pipefail

compose_cmd() {
  if command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    docker compose "$@"
  fi
}

SCRIPT_DIR="$(dirname "$0")"
pushd "$SCRIPT_DIR" >/dev/null

compose_cmd up -d --wait

QUEUE_ENDPOINT="http://localhost:4566/"
payload="Action=CreateQueue&QueueName=embedding-jobs&Version=2012-11-05"
curl -s -o /dev/null -X POST "$QUEUE_ENDPOINT" -d "$payload"

PG_URI="postgres://postgres:password@localhost:5433/revtest?sslmode=disable"
psql "$PG_URI" -v ON_ERROR_STOP=1 -f schema.sql

psql "$PG_URI" <<'SQL'
insert into asc_paragraphs (
    id, framework, topic, asc_reference, guidance_version,
    source_type, authority_score, source_id, schema_version,
    content, embedding_status
) values (
    '11111111-1111-1111-1111-111111111111',
    'US_GAAP',
    'ASC606',
    'ASC606-10-25-1',
    'ASU2014-09',
    'authoritative',
    1.0,
    'fixture-source',
    'v1.0-2025-10-15',
    'Initial paragraph for integration test.',
    'pending'
) on conflict (id) do nothing;
SQL

popd >/dev/null

echo "Integration environment ready."
