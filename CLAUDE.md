# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

GophDrive is a serverless Markdown notes app: Next.js SPA on S3+CloudFront, single Go Lambda behind API Gateway, notes stored in the user's own Google Drive, OAuth refresh tokens encrypted with KMS in DynamoDB.

## Mandatory Post-Change Checklist

1. **Run `./scripts/check.sh`** (fmt + vet + test + prettier + tsc + eslint + vitest across all stacks) and ensure it passes. Runs entirely on the host via `mise x` — no DynamoDB Local / overmind required.
2. **If `core/` changed** → `frontend/public/core.wasm` must be regenerated. The `wasm` overmind process does this automatically while `dev.sh` / `overmind start` is up; otherwise run `./scripts/internal/build-wasm.sh`.
3. **If `frontend/src/`, `core/`, or `frontend/public/` changed** → bump `CACHE_NAME` in `frontend/public/sw.js` to `gophdrive-YYYYMMDD-NN`. The PWA service worker is only re-evaluated when `sw.js` changes by ≥1 byte; skipping this leaves users on stale code or a JS↔Wasm signature mismatch. Mention the bump in the commit message (e.g. `feat: ... and PWA cache v20260508-01`).

## Common Commands

Local dev runs natively on the host. Toolchain versions come from `.tool-versions` via [mise](https://mise.jdx.dev/). The only Docker piece is **DynamoDB Local** (everything else AWS — KMS / SSM / Lambda / API Gateway — is bypassed in `DEV_MODE=true` via `MockEncryptor` + `EnvResolver` + `cmd/server`). Process orchestration is [overmind](https://github.com/DarthSim/overmind) reading the root `Procfile`.

Required CLIs on the host: `mise`, `overmind`, `docker`, `aws`.

```bash
./scripts/setup.sh                # First-time: mise install, npm ci, DynamoDB Local + tables, .env
./scripts/dev.sh                  # Boot DynamoDB Local + overmind start (backend/frontend/wasm)
./scripts/check.sh                # REQUIRED after any change
./scripts/tests.sh                # All unit suites on host (no DynamoDB Local needed)
./scripts/internal/build-wasm.sh  # Manual core/ → core.wasm rebuild
./scripts/deploy-aws.sh           # Production deploy (needs GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET)
```

`overmind connect <backend|frontend|wasm>` attaches to a single process for log inspection or restart (`Ctrl-c` inside, then `overmind restart <name>` from another shell).

### Single-test invocation

```bash
mise x go   -- bash -c 'cd backend  && go test ./internal/handler -run TestName -count=1'
mise x go   -- bash -c 'cd core     && go test ./markdown        -run TestName -count=1'
mise x node -- bash -c 'cd frontend && npx vitest run src/lib/api.test.ts'
mise x node -- bash -c 'cd infra    && npx vitest run test/compute-stack.test.ts'
```

## Architecture

### Build targets

| Path        | Module / package                    | Output                                               |
| ----------- | ----------------------------------- | ---------------------------------------------------- |
| `backend/`  | `github.com/jun/gophdrive/backend`  | Linux ARM64 `bootstrap` for `provided.al2023` Lambda |
| `core/`     | `github.com/jun/gophdrive/core`     | `frontend/public/core.wasm` (`GOOS=js GOARCH=wasm`)  |
| `frontend/` | `gophdrive-frontend` (Next.js)      | Static export to `frontend/out/`                     |
| `infra/`    | `gophdrive-infra` (AWS CDK v2 / TS) | CloudFormation stacks                                |

`core/` is consumed by the frontend as Wasm only — the backend does not import it. `core/bridge/main_wasm.go` registers `renderMarkdown`, `checkConflict`, `createOfflineChange` on `window`. `wasm_exec.js` is copied from the active Go SDK's `GOROOT` by `build-wasm.sh`, so re-run it after a Go version change.

### Single-Lambda router

`backend/cmd/api/main.go` boots `app.NewApp` and starts `lambda.Start(app.HandleRequest)`. **All routing lives in `backend/internal/app/app.go`** as a hand-written `if/else` chain over `req.Path` × `req.HTTPMethod`. To add a route: edit `app.go` and the matching `internal/handler/*.go`. There is no framework or generated routing.

The router strips a leading `/api` prefix because CloudFront proxies `/api/*` to API Gateway. In production every request must carry `X-Origin-Verify: <api-gateway-secret>` (set by a CloudFront origin request policy); the check is bypassed when `DEV_MODE=true`.

### Storage adapter pattern (load-bearing abstraction)

`backend/internal/adapter/storage.go` defines `StorageAdapter` (file CRUD, folders, starred, recent, search). All handlers depend on the interface only. Two implementations:

- `adapter/googledrive/` — real Google Drive API using OAuth tokens decrypted via KMS.
- `adapter/memory/` — DynamoDB-backed (`FileStore` table) for demo users and for `DEV_MODE=true`.

`StorageProvider.GetAdapter(ctx, userID)` returns the right adapter per request. In production `app.HybridProvider` routes user IDs prefixed with `demo-user-` to memory and everyone else to Google Drive. To add a backing store, implement `StorageAdapter` in a new package under `adapter/` and wire it through a provider.

### Conflict / locking model — two layers

1. **Pessimistic session lock** — `backend/internal/session` writes to DynamoDB `EditingSessions` (TTL via `expires_at`). Frontend calls `/sessions/{fileId}/lock` and `/sessions/{fileId}/heartbeat`.
2. **Optimistic ETag** — `StorageAdapter.SaveFile` requires the prior ETag; mismatch surfaces as conflict and the frontend uses Wasm `checkConflict` to resolve.

### Secrets, KMS, DynamoDB

`internal/secret.Resolver` chooses between:

- **Production** `SSMResolver` → SSM Parameter Store keys `/gophdrive/jwt-secret`, `/gophdrive/google-client-secret`, `/gophdrive/api-gateway-secret`. `deploy-aws.sh` auto-generates JWT and gateway secrets on first deploy.
- **`DEV_MODE=true`** `EnvResolver` reads env vars directly (loaded by overmind from the root `.env`); KMS is replaced by `crypto.MockEncryptor`. No SSM, no LocalStack — DynamoDB Local is the only AWS surface.

DynamoDB tables: `UserTokens` (encrypted refresh tokens, key=`user_id`), `EditingSessions` (key=`file_id`, TTL=`expires_at`), `FileStore` (memory-adapter persistence, key=`pk`).

### CDK stacks (`infra/lib/`)

`security-stack` (KMS + IAM) → `database-stack` (3 DynamoDB tables) → `compute-stack` (Lambda + API Gateway) → `frontend-stack` (S3 + CloudFront with `X-Origin-Verify` origin policy and a CloudFront Function that appends `index.html` to directory paths for SPA fallback). `bin/infra.ts` wires them.
