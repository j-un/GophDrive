---
name: verify
description: Build/launch/drive recipe for verifying GophDrive changes end-to-end on the host (backend HTTP surface + gophmem CLI surface) without the full overmind stack.
---

# GophDrive end-to-end verify recipe

## Minimal backend handle (no overmind, no frontend)

1. Docker daemon must be up (Rancher Desktop: `rdctl start`, then poll `docker ps`).
2. `docker compose up -d dynamodb-local` (port 8000; tables persist in the `dynamodb-data` volume — `FileStore`, `EditingSessions`, `APIKeyHashes` normally already exist from setup.sh).
3. Launch the dev server:
   ```bash
   set -a; source .env; set +a
   export AWS_ENDPOINT_URL=http://127.0.0.1:8000   # NOT localhost — Go resolves it to ::1 and Rancher publishes IPv4 only
   cd backend && mise x go -- go run ./cmd/server   # serves :8080
   ```

## Getting credentials

- **Cookie session (web surface)**: `curl -s -D- -o /dev/null http://localhost:8080/auth/demo-login` → copy the `session_token` Set-Cookie value. Bearer JWT does NOT work — token extraction is cookie-only.
- **API key (agent/gophmem surface)**: demo users cannot issue keys via POST /api-keys (403). Insert a row directly into local DynamoDB instead:
  pk = hex sha256 of a chosen plaintext, plus `user_id`, `base_folder_id` (both from `/auth/user` with the cookie), `created_at` (epoch), `key_prefix`. Then `Authorization: Bearer <plaintext>` works via translateAPIKey. Delete the row when done.

## Driving the gophmem surface

```bash
cd tools/gophmem && mise x go -- go build -o /tmp/gophmem .   # build from branch
export GOPHMEM_BASE_URL=http://localhost:8080 GOPHMEM_API_KEY=<plaintext>
```
Then drive write/search/links/backlinks/graph/unresolved as a user would. Demo-user notes TTL out after 60 min, so test data self-cleans; the API-key row does not (delete it manually).

## Gotchas

- `gophmem write --tags` prepends its own frontmatter block — when testing frontmatter parsing (type/aliases/status), put tags inside the heredoc frontmatter and omit `--tags`.
- The stray `tools/gophmem/gophmem` binary from `go build` must not be committed.
- Kill the `go run ./cmd/server` process when finished (`lsof -iTCP:8080 -sTCP:LISTEN`).
