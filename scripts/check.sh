#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Define colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Starting GophDrive Code Quality Checks ===${NC}\n"

# Check if Docker compose is running
if ! docker compose ps | grep -q 'Up'; then
  echo "Development containers are not running. Please start them with ./scripts/dev.sh first."
  exit 1
fi

echo -e "${GREEN}==> Checking Backend (Go) <==${NC}"
echo "Running go fmt..."
docker compose exec -T backend go fmt ./...
echo "Running go vet..."
docker compose exec -T backend go vet ./...
echo "Running backend unit tests..."
docker compose exec -T backend go test ./...

echo -e "\n${GREEN}==> Checking Core (Go/Wasm) <==${NC}"
echo "Running go fmt..."
docker compose exec -T -w /workspace/core air-wasm go fmt ./...
echo "Running go vet..."
docker compose exec -T -w /workspace/core air-wasm go vet ./...
echo "Running core unit tests..."
docker compose exec -T -w /workspace/core air-wasm go test ./...

echo -e "\n${GREEN}==> Checking Frontend (Next.js/TS) <==${NC}"
echo "Running prettier (auto-format)..."
docker compose exec -T frontend npx --yes prettier --write .
echo "Running eslint..."
docker compose exec -T frontend npm run lint
echo "Running frontend unit tests..."
docker compose exec -T frontend npm run test

echo -e "\n${GREEN}==> Checking Infrastructure (AWS CDK/TS) <==${NC}"
echo "Running prettier (auto-format)..."
docker compose exec -T -w /workspace/infra infra npx --yes prettier --write .
echo "Running eslint..."
docker compose exec -T -w /workspace/infra infra npm run lint

echo -e "\n${GREEN}=== All checks passed successfully! ===${NC}"
