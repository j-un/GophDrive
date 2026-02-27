#!/bin/bash
set -euo pipefail

echo "🏗 Building Go/Wasm Core..."
cd core

GOOS=js GOARCH=wasm go build -o ../frontend/public/core.wasm ./bridge/main_wasm.go

# Copy wasm_exec.js
WASM_EXEC_PATH=$(find "$(go env GOROOT)" -name wasm_exec.js | head -n 1)
if [ -n "$WASM_EXEC_PATH" ]; then
    cp "$WASM_EXEC_PATH" ../frontend/public/
    echo "✅ Copied wasm_exec.js from $WASM_EXEC_PATH"
else
    echo "❌ Error: wasm_exec.js not found in GOROOT ($(go env GOROOT))"
    exit 1
fi

echo "✅ Wasm build complete: frontend/public/core.wasm"
