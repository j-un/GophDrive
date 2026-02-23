#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# run-tests.sh — Launch the test-runner container via docker compose
# =============================================================================

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "$PROJECT_DIR"

export GO_VERSION=$(cat .go-version)
export NODE_VERSION=$(cat .node-version)

echo "Building and running test-runner container (Go: $GO_VERSION, Node: $NODE_VERSION)..."
docker compose run --rm --build test-runner
