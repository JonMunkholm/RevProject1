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

: "${AWS_TEST_REGION:=us-west-1}"
QUEUE_ENDPOINT="http://localhost:4566"

awslocal() {
  AWS_REGION="$AWS_TEST_REGION" \
  AWS_DEFAULT_REGION="$AWS_TEST_REGION" \
  AWS_ACCESS_KEY_ID=test \
  AWS_SECRET_ACCESS_KEY=test \
  aws --region "$AWS_TEST_REGION" --endpoint-url "$QUEUE_ENDPOINT" "$@"
}

QUEUE_NAME="embedding-jobs"
DLQ_NAME="${QUEUE_NAME}-dead-letter"

awslocal sqs create-queue --queue-name "$DLQ_NAME" >/dev/null 2>&1 || true
awslocal sqs create-queue --queue-name "$QUEUE_NAME" >/dev/null 2>&1 || true

QUEUE_URL=$(awslocal sqs get-queue-url --queue-name "$QUEUE_NAME" --query 'QueueUrl' --output text)
DLQ_URL=$(awslocal sqs get-queue-url --queue-name "$DLQ_NAME" --query 'QueueUrl' --output text)
DLQ_ARN=$(awslocal sqs get-queue-attributes --queue-url "$DLQ_URL" --attribute-names QueueArn --query 'Attributes.QueueArn' --output text)

awslocal sqs set-queue-attributes \
  --queue-url "$QUEUE_URL" \
  --attributes RedrivePolicy="{\"deadLetterTargetArn\":\"$DLQ_ARN\",\"maxReceiveCount\":\"5\"}" >/dev/null 2>&1 || true

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
