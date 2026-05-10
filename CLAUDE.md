# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

GophDrive is a serverless Markdown notes app: Next.js SPA on S3+CloudFront, single Go Lambda behind API Gateway, notes stored in DynamoDB (`FileStore` table). Google is used purely as an OIDC identity provider — no Drive scope, no stored OAuth refresh tokens.

## Mandatory Post-Change Checklist

1. **Run `./scripts/check.sh`** (fmt + vet + test + prettier + tsc + eslint + vitest across all stacks) and ensure it passes. Runs entirely on the host via `mise x` — no DynamoDB Local / overmind required.
2. **If `core/` changed** → `frontend/public/core.wasm` must be regenerated. The `wasm` overmind process does this automatically while `dev.sh` / `overmind start` is up; otherwise run `./scripts/internal/build-wasm.sh`.
3. **If `frontend/src/`, `core/`, or `frontend/public/` changed** → bump `CACHE_NAME` in `frontend/public/sw.js` to `gophdrive-YYYYMMDD-NN`. The PWA service worker is only re-evaluated when `sw.js` changes by ≥1 byte; skipping this leaves users on stale code or a JS↔Wasm signature mismatch. Mention the bump in the commit message (e.g. `feat: ... and PWA cache v20260508-01`).

## Common Commands

Local dev runs natively on the host. Toolchain versions come from `.tool-versions` via [mise](https://mise.jdx.dev/). The only Docker piece is **DynamoDB Local** (SSM / Lambda / API Gateway are bypassed in `DEV_MODE=true` via `EnvResolver` + `cmd/server`). Process orchestration is [overmind](https://github.com/DarthSim/overmind) reading the root `Procfile`.

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

The default `.env` ships dummy Google OAuth credentials, so the live login flow is bypassed locally — Demo login (`/auth/demo-login`) covers most code paths. To exercise the real OAuth path (ID token verification, email allow-list, cookie round-trip), see [`docs/local-google-oauth.md`](docs/local-google-oauth.md).

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

`backend/internal/adapter/storage.go` defines `StorageAdapter` (file CRUD, folders, starred, recent, search, export). All handlers depend on the interface only. The single implementation today is `adapter/dynamo/` — every user (demo and Google-authenticated) reads/writes the same `FileStore` table. Demo users are gated by a `demo-user-` prefix on `userID`: that prefix toggles a 60-minute DDB TTL and a 50-item cap; real users are uncapped and never expire.

`StorageProvider.GetAdapter(ctx, userID, baseFolderID)` returns the per-user adapter. To add a backing store, implement `StorageAdapter` in a new package under `adapter/` and route through a provider.

Body storage is split into two mutually-exclusive fields on each row (`content` inline vs `body_s3_key`) so a future spillover path for image/binary uploads can flip without re-versioning the schema. Today only the inline path is exercised; `routeBody` is the single decision point.

### Conflict / locking model — two layers

1. **Pessimistic session lock** — `backend/internal/session` writes to DynamoDB `EditingSessions` (TTL via `expires_at`). Frontend calls `/sessions/{fileId}/lock` and `/sessions/{fileId}/heartbeat`.
2. **Optimistic ETag** — `StorageAdapter.SaveFile` requires the prior ETag; mismatch surfaces as conflict and the frontend uses Wasm `checkConflict` to resolve.

### Auth flow

Google OIDC only (scopes: `openid email profile`). `Callback` exchanges the code, calls `auth.GoogleVerifier.Verify` against Google's JWKS, then auto-mints a per-user "GophDrive" root folder via `EnsureRootFolder`. The session is a self-issued JWT (HS256) carrying `sub`, `email`, `name`, `base_folder_id`, `exp` — no DB lookup needed on subsequent requests.

`Refresh` re-signs at `SessionTTL` (24h) preserving claims, **except** for demo users (`sub` starts with `demo-user-`) — those return 401 to keep the demo flow's short-lived semantics. The frontend `apiFetch` 401-handler then clears the token cleanly.

`DemoLogin` mints a `demo-user-<uuid>` JWT at `DemoSessionTTL` (1h), seeds a fresh root folder + welcome notes, and redirects with the token in the query string.

For exercising the live OAuth path on localhost, see [`docs/local-google-oauth.md`](docs/local-google-oauth.md).

### Secrets, DynamoDB, S3

`internal/secret.Resolver` chooses between:

- **Production** `SSMResolver` → SSM Parameter Store keys `/gophdrive/jwt-secret`, `/gophdrive/google-client-secret`, `/gophdrive/api-gateway-secret`. `deploy-aws.sh` auto-generates JWT and gateway secrets on first deploy.
- **`DEV_MODE=true`** `EnvResolver` reads env vars directly (loaded by overmind from the root `.env`). DynamoDB Local is the only AWS surface; SSM and the other AWS services are bypassed.

DynamoDB tables (2):
- `EditingSessions` — key=`file_id`, TTL=`expires_at`. Pessimistic edit-session locks. Ephemeral; PITR off.
- `FileStore` — key=`pk`. Notes and folders. **PITR enabled**, `RemovalPolicy.RETAIN`. Demo users have TTL=`ttl` populated (60 min); real users have `ttl` omitted entirely (`omitempty`).

S3 bucket: `BodyStoreBucket` (provisioned in compute-stack, `RemovalPolicy.RETAIN`, SSE-S3, public access blocked). Reserved for the future S3 spillover read/write path; the current code only references its name via the `BODY_STORE_BUCKET` env var and never reads/writes objects.

### CDK stacks (`infra/lib/`)

`database-stack` (2 DynamoDB tables) → `compute-stack` (Lambda + API Gateway + BodyStore S3 bucket) → `frontend-stack` (S3 + CloudFront with `X-Origin-Verify` origin policy and a CloudFront Function that appends `index.html` to directory paths for SPA fallback). `bin/infra.ts` wires them.

### Disaster recovery

Phase 6 enabled DynamoDB PITR on `FileStore` (35-day any-second restore) and added `Export(ctx)` on `StorageAdapter` plus `/api/export` returning a ZIP of every note. Recovery procedures (PITR restore, cold-start from a ZIP) live in [`docs/disaster-recovery.md`](docs/disaster-recovery.md).
