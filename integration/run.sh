#!/usr/bin/env bash
set -euo pipefail

ROOT="$(dirname "$0")/.."
export $(grep -v '^#' "$ROOT/integration/config.env" | xargs -d '\n')

AWS_TEST_REGION=${AWS_TEST_REGION:-us-west-1}
PG_URI=${DB_URL}
QUEUE_URL=${EMBED_QUEUE_URL}

cleanup() {
  echo "Stopping worker..."
  kill $WORKER_PID 2>/dev/null || true
}
trap cleanup EXIT

pushd "$ROOT" >/dev/null

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
) on conflict (id) do nothing;
SQL

psql "$PG_URI" < tmp/fixture.sql
psql "$PG_URI" <<'SQL'
delete from asc_embeddings where paragraph_id = '11111111-1111-1111-1111-111111111111';
SQL

WORKER_LOG=tmp/worker.log
(
  AWS_REGION="$AWS_TEST_REGION" \
  AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
  AWS_ENDPOINT_URL_SQS="http://localhost:4566" \
  AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test \
  DB_URL="$DB_URL" \
  EMBED_QUEUE_URL="$QUEUE_URL" \
  OPENAI_API_KEY="$OPENAI_API_KEY" \
  OPENAI_PROJECT_ID="$OPENAI_PROJECT_ID" \
  OPENAI_API_BASE="$OPENAI_API_BASE" \
  go run ./cmd/worker > "$WORKER_LOG" 2>&1
) &
WORKER_PID=$!

sleep 3

python3 - "$QUEUE_URL" <<'PY'
import json, sys, urllib.parse, urllib.request

queue_url = sys.argv[1]
payload = {
    "job_id": "22222222-2222-2222-2222-222222222222",
    "paragraph_id": "11111111-1111-1111-1111-111111111111",
    "source_hash": "fixture-source",
    "model": "text-embedding-3-large",
    "priority": "normal",
    "metadata_version": "v1.0-2025-10-15",
    "created_at": "2025-01-01T00:00:00Z"
}

params = urllib.parse.urlencode({
    "Action": "SendMessage",
    "QueueUrl": queue_url,
    "MessageBody": json.dumps(payload),
    "Version": "2012-11-05"
}).encode("utf-8")

urllib.request.urlopen("http://localhost:4566/", params).read()
PY

sleep 5

psql "$PG_URI" <<'SQL'
copy (
  select status, attempts, last_error, completed_at
  from embedding_jobs where id = '22222222-2222-2222-2222-222222222222'
) to stdout;
select count(*) from asc_embeddings where paragraph_id = '11111111-1111-1111-1111-111111111111';
SQL

cat <<'REPORT'
--- Worker Logs ---
REPORT
cat "$WORKER_LOG"

cat <<'REPORT'
--- Done ---
REPORT

popd >/dev/null
