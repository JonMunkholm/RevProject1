#!/usr/bin/env bash
set -euo pipefail

ROOT="$(dirname "$0")"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command '$1' not found on PATH" >&2
    exit 1
  fi
}

require_cmd docker
if ! docker info >/dev/null 2>&1; then
  echo "error: docker daemon is not available; start Docker Desktop or dockerd" >&2
  exit 1
fi

if ! command -v localstack >/dev/null 2>&1; then
  echo "localstack CLI not found; installing via pip..."
  if ! command -v pip3 >/dev/null 2>&1; then
    echo "error: pip3 not found; install Python 3 and pip to proceed" >&2
    exit 1
  fi
  pip3 install --upgrade localstack
fi

IMAGE="localstack/localstack:latest"
echo "Pulling LocalStack image $IMAGE..."
docker pull "$IMAGE"

echo "Launching integration environment..."
exec "$ROOT/setup.sh"
