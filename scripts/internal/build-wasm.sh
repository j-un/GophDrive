#!/bin/bash
set -euo pipefail

echo "🏗 Building Go/Wasm Core..."
cd core

GOOS=js GOARCH=wasm go build -o ../frontend/public/core.wasm ./bridge/main_wasm.go

# Copy wasm_exec.js
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ../frontend/public/

echo "✅ Wasm build complete: frontend/public/core.wasm"
