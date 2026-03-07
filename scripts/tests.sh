#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# run-tests.sh — Launch the test-runner container via docker compose
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "$PROJECT_DIR"

if [ -f .tool-versions ]; then
  export GO_VERSION=$(grep '^golang ' .tool-versions | awk '{print $2}' | tr -d '[:space:]')
  export NODE_VERSION=$(grep '^nodejs ' .tool-versions | awk '{print $2}' | tr -d '[:space:]')
fi

echo "Building and running test-runner container (Go: $GO_VERSION, Node: $NODE_VERSION)..."
docker compose run --rm --build test-runner
