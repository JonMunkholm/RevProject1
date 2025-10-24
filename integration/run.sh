#!/usr/bin/env bash
set -euo pipefail

ROOT="$(dirname "$0")/.."
export $(grep -v '^#' "$ROOT/integration/config.env" | xargs -d '\n')

GOCACHE_DIR=$(mktemp -d)
export GOCACHE="$GOCACHE_DIR"

: "${AWS_TEST_REGION:=us-west-1}"
PG_URI=${DB_URL}
QUEUE_URL=${EMBED_QUEUE_URL}
DLQ_URL=${EMBED_DLQ_URL}
QUEUE_ENDPOINT="http://localhost:4566"
MAX_ATTEMPTS=${EMBED_MAX_ATTEMPTS:-3}
if ! [[ "$MAX_ATTEMPTS" =~ ^[0-9]+$ ]] || [[ "$MAX_ATTEMPTS" -le 0 ]]; then
  MAX_ATTEMPTS=3
fi

awslocal() {
  AWS_REGION="$AWS_TEST_REGION" \
  AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
  AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test \
  aws --region "$AWS_TEST_REGION" --endpoint-url "$QUEUE_ENDPOINT" "$@"
}

wait_for_status() {
  local job_id="$1"
  local expected="$2"
  local retries="${3:-10}"
  local status=""
  for ((i=1; i<=retries; i++)); do
    status=$(psql "$PG_URI" -t -A -c "select status from embedding_jobs where id = '$job_id';" | tr -d '[:space:]')
    if [[ "$status" == "$expected" ]]; then
      return 0
    fi
    sleep 2
  done
  echo "Timed out waiting for job $job_id to reach status $expected (last status: ${status:-<none>})"
  return 1
}

awslocal sqs purge-queue --queue-url "$QUEUE_URL" >/dev/null 2>&1 || true
if [[ -n "${DLQ_URL:-}" ]]; then
  awslocal sqs purge-queue --queue-url "$DLQ_URL" >/dev/null 2>&1 || true
fi

cleanup() {
  echo "Stopping worker..."
  kill ${WORKER_PID-0} 2>/dev/null || true
  echo "Stopping API..."
  stop_api
  rm -rf "${GOCACHE_DIR:-}"
}
trap cleanup EXIT

pushd "$ROOT" >/dev/null

psql "$PG_URI" <<'SQL'
delete from asc_embeddings;
SQL

mkdir -p tmp
cat <<'SQL' > tmp/fixture.sql
insert into embedding_jobs (
  id,
  paragraph_id,
  source_hash,
  model,
  priority,
  metadata_version
) values (
  '22222222-2222-2222-2222-222222222222',
  '11111111-1111-1111-1111-111111111111',
  'fixture-source',
  'text-embedding-3-large',
  'normal',
  'v1.0-2025-10-15'
) on conflict (id) do update
set paragraph_id = excluded.paragraph_id,
    status = 'pending',
    attempts = 0,
    last_error = null,
    completed_at = null,
    updated_at = now();

insert into embedding_jobs (
  id,
  paragraph_id,
  source_hash,
  model,
  priority,
  metadata_version
) values (
  '33333333-3333-3333-3333-333333333333',
  'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
  'failure-fixture-source',
  'text-embedding-3-small',
  'normal',
  'v1.0-2025-10-15'
) on conflict (id) do update
set paragraph_id = excluded.paragraph_id,
    status = 'pending',
    attempts = 0,
    last_error = null,
    completed_at = null,
    updated_at = now();
SQL

psql "$PG_URI" < tmp/fixture.sql
psql "$PG_URI" <<'SQL'
delete from asc_embeddings where paragraph_id = '11111111-1111-1111-1111-111111111111';
SQL

WORKER_LOG=tmp/worker.log
(
  AWS_REGION="$AWS_TEST_REGION" \
  AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
  AWS_ENDPOINT_URL_SQS="$QUEUE_ENDPOINT" \
  AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test \
  DB_URL="$DB_URL" \
  EMBED_QUEUE_URL="$QUEUE_URL" \
  EMBED_DLQ_URL="$DLQ_URL" \
  EMBED_MAX_ATTEMPTS="${EMBED_MAX_ATTEMPTS:-3}" \
  OPENAI_API_KEY="$OPENAI_API_KEY" \
  OPENAI_PROJECT_ID="$OPENAI_PROJECT_ID" \
  OPENAI_API_BASE="$OPENAI_API_BASE" \
  WORKER_METRICS_ADDR="127.0.0.1:0" \
  go run ./cmd/worker > "$WORKER_LOG" 2>&1
) &
WORKER_PID=$!

sleep 3

cat <<'JSON' > tmp/message-success.json
{
  "job_id": "22222222-2222-2222-2222-222222222222",
  "paragraph_id": "11111111-1111-1111-1111-111111111111",
  "source_hash": "fixture-source",
  "model": "text-embedding-3-large",
  "priority": "normal",
  "metadata_version": "v1.0-2025-10-15",
  "created_at": "2025-01-01T00:00:00Z"
}
JSON

(
  AWS_REGION="$AWS_TEST_REGION" \
  AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
  AWS_ENDPOINT_URL_SQS="$QUEUE_ENDPOINT" \
  AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test \
  EMBED_QUEUE_URL="$QUEUE_URL" \
  go run ./cmd/enqueue -body-file tmp/message-success.json
)

wait_for_status '22222222-2222-2222-2222-222222222222' 'succeeded'

cat <<'JSON' > tmp/message-failure.json
{
  "job_id": "33333333-3333-3333-3333-333333333333",
  "paragraph_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
  "source_hash": "failure-fixture-source",
  "model": "text-embedding-3-small",
  "priority": "normal",
  "metadata_version": "v1.0-2025-10-15",
  "created_at": "2025-01-01T00:00:00Z"
}
JSON

for attempt in $(seq 1 "$MAX_ATTEMPTS"); do
  echo "Triggering failure attempt ${attempt}/${MAX_ATTEMPTS}..."
  (
    AWS_REGION="$AWS_TEST_REGION" \
    AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
    AWS_ENDPOINT_URL_SQS="$QUEUE_ENDPOINT" \
    AWS_ACCESS_KEY_ID=test \
    AWS_SECRET_ACCESS_KEY=test \
    EMBED_QUEUE_URL="$QUEUE_URL" \
    go run ./cmd/enqueue -body-file tmp/message-failure.json
  )
  sleep 2
done

wait_for_status '33333333-3333-3333-3333-333333333333' 'dead_letter'

psql "$PG_URI" <<'SQL'
\echo '-- Job Statuses --'
select id, status, attempts, coalesce(last_error,'') as last_error, completed_at
from embedding_jobs
where id in ('22222222-2222-2222-2222-222222222222','33333333-3333-3333-3333-333333333333')
order by id;

\echo '-- Paragraph Embedding Status --'
select id, embedding_status
from asc_paragraphs
where id = '11111111-1111-1111-1111-111111111111';

\echo '-- Embedding Count --'
select count(*) as embeddings_for_fixture
from asc_embeddings
where paragraph_id = '11111111-1111-1111-1111-111111111111';
SQL

API_PORT=$(shuf -i 20000-39999 -n 1)
API_ADDR=":${API_PORT}"
API_BASE="http://localhost:${API_PORT}"
API_LOG_DIR=tmp
mkdir -p "$API_LOG_DIR"

wait_for_http() {
  local url="$1"
  local retries="${2:-15}"
  local code
  for ((i=1; i<=retries; i++)); do
    if code=$(curl -s -o /dev/null -w '%{http_code}' "$url"); then
      if [[ "$code" -ge 200 && "$code" -le 299 ]]; then
        return 0
      fi
    fi
    sleep 1
  done
  echo "Timed out waiting for $url to become available"
  return 1
}

expect_http() {
  local expected="$1"
  local url="$2"
  shift 2
  local description="${*:-$url}"
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' "$url")
  if [[ "$code" != "$expected" ]]; then
    echo "Expected HTTP $expected for $description but received $code"
    echo "--- API log ---"
    [[ -f "$API_LOG" ]] && tail -n 40 "$API_LOG"
    exit 1
  fi
  echo "[$code] $description"
}

start_api() {
  local guardrails="$1"
  echo "Starting API on $API_ADDR (guardrails=${guardrails})"
  API_LOG="$API_LOG_DIR/api_guardrails_${guardrails}.log"
  API_GOCACHE_DIR="$(mktemp -d)"
  GUARDRAILS_ENABLED="$guardrails" \
  DB_URL="$DB_URL" \
  OPENAI_API_KEY="$OPENAI_API_KEY" \
  OPENAI_API_BASE="$OPENAI_API_BASE" \
  OPENAI_PROJECT_ID="$OPENAI_PROJECT_ID" \
  PORT="$API_ADDR" \
  GOCACHE="$API_GOCACHE_DIR" go run ./cmd/api > "$API_LOG" 2>&1 &
  API_PID=$!
  wait_for_http "$API_BASE/api/search?q=guardrail+ping"
}

stop_api() {
  if [[ -n "${API_PID:-}" ]]; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
    unset API_PID
  fi
  if [[ -n "${API_GOCACHE_DIR:-}" ]]; then
    rm -rf "$API_GOCACHE_DIR"
    unset API_GOCACHE_DIR
  fi
}

echo "--- Guardrail checks (disabled) ---"
start_api false
expect_http 200 "$API_BASE/api/search?q=revenue" "default search with guardrails disabled"
expect_http 400 "$API_BASE/api/search?q=revenue&includeInternal=true" "includeInternal rejected when guardrails disabled"
stop_api

echo "--- Guardrail checks (enabled) ---"
start_api true
expect_http 200 "$API_BASE/api/search?q=revenue" "default search with guardrails enabled"
expect_http 400 "$API_BASE/api/search?q=revenue&includeInternal=true" "includeInternal requires tenant context without capability"
expect_http 400 "$API_BASE/api/search?q=revenue&asOf=2024-01-01" "asOf requires capability"
stop_api

cat <<'REPORT'
--- Worker Logs ---
REPORT
cat "$WORKER_LOG"

if [[ -n "${DLQ_URL:-}" ]]; then
  cat <<'REPORT'
--- DLQ Attributes ---
REPORT
  awslocal sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names ApproximateNumberOfMessages ApproximateNumberOfMessagesNotVisible || true
fi

cat <<'REPORT'
--- Done ---
REPORT

popd >/dev/null
