#!/usr/bin/env bash
# Mandatory post-change checks. Runs everything on the host (no Docker exec).
# Requires `mise install` to have been run (see scripts/setup.sh).
set -euo pipefail

cd "$(dirname "$0")/.."

GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'
section() { echo -e "\n${GREEN}==> $* <==${NC}"; }

echo -e "${BLUE}=== GophDrive Code Quality Checks ===${NC}"

section "Backend (Go)"
mise x go -- bash -c 'cd backend && go fmt ./... && go vet ./... && go test ./...'

section "Core (Go / Wasm)"
mise x go -- bash -c 'cd core && go fmt ./... && go vet ./... && go test ./...'

section "Frontend (Next.js / TS)"
mise x node -- bash -c '
  cd frontend &&
  npx --yes prettier --write . &&
  npx --yes tsc --noEmit &&
  npm run lint &&
  npm run test
'

section "Infrastructure (AWS CDK / TS)"
mise x node -- bash -c '
  cd infra &&
  npx --yes prettier --write . &&
  npm run lint &&
  npx --yes tsc --noEmit &&
  npm run test
'

echo -e "\n${GREEN}=== All checks passed ===${NC}"
