#!/usr/bin/env bash
# Build core/ -> frontend/public/core.wasm. Safe to invoke from any cwd.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT/core"

echo "🏗  Building Go/Wasm Core..."
GOOS=js GOARCH=wasm go build -o "$ROOT/frontend/public/core.wasm" ./bridge/main_wasm.go

WASM_EXEC_PATH=$(find "$(go env GOROOT)" -name wasm_exec.js | head -n 1)
if [ -z "$WASM_EXEC_PATH" ]; then
  echo "❌ wasm_exec.js not found under GOROOT ($(go env GOROOT))" >&2
  exit 1
fi
cp "$WASM_EXEC_PATH" "$ROOT/frontend/public/"

echo "✅ Wasm build complete: frontend/public/core.wasm (wasm_exec.js from $WASM_EXEC_PATH)"
