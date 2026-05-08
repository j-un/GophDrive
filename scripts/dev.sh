#!/usr/bin/env bash
# Boot the local dev stack: ensures DynamoDB Local is up, then runs overmind.
# Run ./scripts/setup.sh once before the first invocation.
set -euo pipefail

cd "$(dirname "$0")/.."

if ! docker compose ps --status running --services | grep -q '^dynamodb-local$'; then
  echo "==> Starting DynamoDB Local"
  docker compose up -d dynamodb-local
fi

exec overmind start
