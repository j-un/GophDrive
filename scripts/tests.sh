#!/usr/bin/env bash
# Run every unit test suite on the host. No DynamoDB Local / overmind required.
set -euo pipefail

cd "$(dirname "$0")/.."

RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

PASS=0; FAIL=0; RESULTS=()

run_suite() {
  local name="$1"; shift
  echo -e "\n${CYAN}${BOLD}━━━ ${name} ━━━${NC}"
  if "$@"; then
    RESULTS+=("${GREEN}✓ ${name}${NC}"); ((PASS++)) || true
  else
    RESULTS+=("${RED}✗ ${name}${NC}"); ((FAIL++)) || true
  fi
}

run_suite "backend (go test)"  mise x go   -- bash -c 'cd backend  && go test ./... -count=1 -cover'
run_suite "core (go test)"     mise x go   -- bash -c 'cd core     && go test ./... -count=1 -cover'
run_suite "frontend (vitest)"  mise x node -- bash -c 'cd frontend && npx vitest run'
run_suite "infra (vitest)"     mise x node -- bash -c 'cd infra    && npx vitest run'

echo -e "\n${BOLD}━━━ Summary ━━━${NC}"
for r in "${RESULTS[@]}"; do echo -e "  $r"; done
echo

if [ "$FAIL" -gt 0 ]; then
  echo -e "${RED}${BOLD}${FAIL} suite(s) failed.${NC}"; exit 1
else
  echo -e "${GREEN}${BOLD}All ${PASS} suites passed.${NC}"
fi
