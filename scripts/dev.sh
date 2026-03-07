#!/bin/bash
set -e

echo "🐳 Setting up Pure Docker Environment..."

# 1. Source versions from files
if [ -f .tool-versions ]; then
  export NODE_VERSION=$(grep '^nodejs ' .tool-versions | awk '{print $2}' | tr -d '[:space:]')
  echo "🔹 Using Node.js version $NODE_VERSION (from .tool-versions)"
  export GO_VERSION=$(grep '^golang ' .tool-versions | awk '{print $2}' | tr -d '[:space:]')
  echo "🔹 Using Go version $GO_VERSION (from .tool-versions)"
fi

# 2. Build and Start Containers
echo "🚀 Building and starting containers..."
docker compose up -d --build

# 2. Wait for LocalStack
echo "⏳ Waiting for LocalStack (in container)..."
# We check from host perspective (localhost:4566) since it's mapped
while ! curl -s http://localhost:4566/_localstack/health | grep -E '"dynamodb": "(available|running)"'; do
  sleep 2
  echo "   Still waiting for LocalStack..."
done
echo "✅ LocalStack is ready."

# 3. Deploy Infrastructure via Infra Container
echo "📦 Deploying Infrastructure..."
docker compose exec infra ./scripts/internal/deploy-local.sh

echo "🎉 Environment is ready!"
echo "   - Frontend: http://localhost:3000"
echo "   - Backend:  http://localhost:8080"
echo "   - LocalStack: http://localhost:4566"
