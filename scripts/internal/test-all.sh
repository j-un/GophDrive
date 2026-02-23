#!/usr/bin/env bash
set -euo pipefail

# =============================================================================
# test-all.sh — Run all unit tests inside the test-runner container
# =============================================================================

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

PASS=0
FAIL=0
RESULTS=()

run_suite() {
  local name="$1"
  shift
  echo ""
  echo -e "${CYAN}${BOLD}━━━ ${name} ━━━${RESET}"
  if "$@"; then
    RESULTS+=("${GREEN}✓ ${name}${RESET}")
    ((PASS++)) || true
  else
    RESULTS+=("${RED}✗ ${name}${RESET}")
    ((FAIL++)) || true
  fi
}

# ---------------------------------------------------------------------------
# 1. Backend (Go)
# ---------------------------------------------------------------------------
run_suite "backend (go test)" \
  sh -c "cd /workspace/backend && go test ./... -count=1 -cover"

# ---------------------------------------------------------------------------
# 2. Core (Go)
# ---------------------------------------------------------------------------
run_suite "core (go test)" \
  sh -c "cd /workspace/core && go test ./... -count=1 -cover"

# ---------------------------------------------------------------------------
# 3. Frontend (Vitest)
# ---------------------------------------------------------------------------
run_suite "frontend (vitest)" \
  sh -c "cd /workspace/frontend && npm ci --silent && npx vitest run --coverage"

# ---------------------------------------------------------------------------
# 4. Infra (Vitest)
# ---------------------------------------------------------------------------
run_suite "infra (vitest)" \
  sh -c "cd /workspace/infra && npm ci --silent && npx vitest run --coverage"

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}━━━ Summary ━━━${RESET}"
for r in "${RESULTS[@]}"; do
  echo -e "  $r"
done
echo ""

if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}${BOLD}${FAIL} suite(s) failed.${RESET}"
  exit 1
else
  echo -e "${GREEN}${BOLD}All ${PASS} suites passed!${RESET}"
  exit 0
fi
