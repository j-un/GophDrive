#!/usr/bin/env bash
# Initial setup for the host-based dev environment.
# Idempotent — safe to re-run.
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"

GREEN='\033[0;32m'; BLUE='\033[0;34m'; YELLOW='\033[0;33m'; NC='\033[0m'
say() { echo -e "${BLUE}==>${NC} $*"; }
ok()  { echo -e "${GREEN}  ok${NC} $*"; }
warn(){ echo -e "${YELLOW}  !!${NC} $*"; }

# ---------------------------------------------------------------------------
# 1. Required CLIs
# ---------------------------------------------------------------------------
say "Checking required CLIs"
for bin in mise overmind docker aws; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    warn "$bin not found — please install (brew install $bin)"
    exit 1
  fi
done
ok "mise / overmind / docker / aws present"

# ---------------------------------------------------------------------------
# 2. Toolchain (Go + Node) via mise
# ---------------------------------------------------------------------------
say "Installing toolchain via mise (.tool-versions)"
mise install go node
ok "go $(mise x go -- go version | awk '{print $3}') / node $(mise x node -- node --version)"

say "Installing air (Go hot reload) into \$GOPATH/bin"
mise x go -- go install github.com/air-verse/air@v1.65.1
GOBIN_DIR="$(mise x go -- bash -c 'echo "${GOBIN:-${GOPATH:-$HOME/go}/bin}"')"
if [ ! -x "$GOBIN_DIR/air" ]; then
  warn "air not found at $GOBIN_DIR/air after install"
  exit 1
fi
ok "air installed at $GOBIN_DIR/air"

# ---------------------------------------------------------------------------
# 3. .env bootstrap
# ---------------------------------------------------------------------------
if [ ! -f "$ROOT/.env" ]; then
  cp "$ROOT/.env.example" "$ROOT/.env"
  ok ".env created from .env.example — edit it before signing in with real Google OAuth"
else
  ok ".env already exists (left untouched)"
fi

# ---------------------------------------------------------------------------
# 4. JS deps
# ---------------------------------------------------------------------------
say "Installing JS dependencies"
mise x node -- bash -c "cd '$ROOT/frontend' && npm ci"
mise x node -- bash -c "cd '$ROOT/infra' && npm ci"
ok "frontend & infra node_modules ready"

# ---------------------------------------------------------------------------
# 5. DynamoDB Local
# ---------------------------------------------------------------------------
say "Starting DynamoDB Local"
docker compose up -d dynamodb-local
echo -n "  waiting for DynamoDB Local on :8000 "
until curl -s -o /dev/null -w "%{http_code}" http://localhost:8000 | grep -qE '^(200|400)$'; do
  echo -n "."
  sleep 1
done
echo
ok "DynamoDB Local ready"

# ---------------------------------------------------------------------------
# 6. Create DynamoDB tables
# ---------------------------------------------------------------------------
say "Creating DynamoDB tables (idempotent)"
# Local DynamoDB doesn't need real credentials. Strip any inherited AWS_PROFILE
# (e.g. SSO profiles) to avoid an unrelated `aws login` prompt.
unset AWS_PROFILE AWS_SESSION_TOKEN
export AWS_PAGER=""
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=ap-northeast-1
AWS_CMD="aws --endpoint-url=http://localhost:8000 --region=ap-northeast-1"

table_exists() { $AWS_CMD dynamodb describe-table --table-name "$1" >/dev/null 2>&1; }

if ! table_exists EditingSessions; then
  $AWS_CMD dynamodb create-table \
    --table-name EditingSessions \
    --attribute-definitions AttributeName=file_id,AttributeType=S \
    --key-schema AttributeName=file_id,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null
  $AWS_CMD dynamodb update-time-to-live \
    --table-name EditingSessions \
    --time-to-live-specification Enabled=true,AttributeName=expires_at >/dev/null || true
  ok "created EditingSessions"
else
  ok "EditingSessions exists"
fi

if ! table_exists FileStore; then
  $AWS_CMD dynamodb create-table \
    --table-name FileStore \
    --attribute-definitions AttributeName=pk,AttributeType=S \
    --key-schema AttributeName=pk,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST >/dev/null
  ok "created FileStore"
else
  ok "FileStore exists"
fi

# ---------------------------------------------------------------------------
# 7. Initial Wasm build (so frontend doesn't 404 on first load before air settles)
# ---------------------------------------------------------------------------
say "Building initial core.wasm"
mise x go -- bash -c "cd '$ROOT' && ./scripts/internal/build-wasm.sh"

echo
echo -e "${GREEN}Setup complete.${NC}"
echo
echo "Next steps:"
echo -e "  ${BLUE}./scripts/dev.sh${NC}            # boots backend(:8080) + frontend(:3000) + wasm watcher"
echo -e "  ${BLUE}overmind connect backend${NC}    # attach to one process (run in another shell)"
echo -e "  ${BLUE}./scripts/check.sh${NC}          # required after any change"
