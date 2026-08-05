# Renovate Dependency Updates

Renovate keeps `go.mod` / `package.json` dependencies current across
`core/`, `backend/`, `tools/gophmem/`, `frontend/`, and `infra/`. Four times
now that automation has landed PRs with drifted manifests that broke CI in
ways that took a manual fix commit to untangle. This doc covers the two failure
classes, the three-layer guard built to prevent/detect/repair them, how to
use `scripts/deps-sync.sh` directly, and the manual runbook for when the
guard doesn't apply.

## The two failure classes

**Go — go.mod indirect drift.**

`backend/go.mod` has `replace github.com/jun/gophdrive/core => ../core`.
Renovate's gomod manager runs `go mod tidy` per-directory and massages
`replace` statements away first, so a bump to `core/go.mod`'s direct
dependencies (goldmark, chroma) is invisible to backend's tidy — backend's
`// indirect` lines go stale. Symptom:

```
go: updates to go.mod needed; to update it: go mod tidy
```

This has hit the repo three times:

- **2026-06-08** — `dlclark/regexp2` → `dlclark/regexp2/v2` module path
  migration (fixed by commit `aa1d4e7`).
- **2026-07-19** — backend's indirect deps left behind by a core bump (fixed
  by commit `728ea07`).
- **2026-08-05** — goldmark `v1.8.2` → `v1.8.5` (PR #168, fixed by commit
  `c25c51d`).

**npm — lockfile drift.**

Renovate's lockfile artifact update fails, leaving `package.json` bumped but
`package-lock.json` stale. Renovate surfaces this as a failing
`renovate/artifacts` check on the PR. Symptom:

```
npm error code EUSAGE
lock file's react@19.2.7 does not satisfy react@19.2.8
```

Occurred **2026-08-05** (PR #169, fixed by commit `e128532`).

Before the guard described below existed, both classes together required
four manual fix commits: `aa1d4e7`, `728ea07`, `c25c51d`, `e128532`.

## Defense in depth

Three layers, in prevent → detect → repair order. None of them alone is
sufficient — each is documented here together with what it cannot do.

### 1. Renovate config (prevention, best-effort)

`.github/renovate.json5`:

- `postUpdateOptions: ['gomodTidy', 'gomodUpdateImportPaths']` —
  `gomodUpdateImportPaths` follows module-path migrations on major upgrades
  (the `dlclark/regexp2` → `dlclark/regexp2/v2` case above).
- A `packageRules` entry re-enables the gomod `indirect` dep type (Renovate
  disables it by default) so that backend's `// indirect` lines get bumped
  in the same grouped branch as core's direct bumps, instead of silently
  falling behind.

Config alone cannot fully solve this: the per-directory `go mod tidy` with
massaged `replace` statements is a structural limitation of how Renovate's
gomod manager works, not something a config flag turns off. Grouping the
bumps into the same branch reduces how often the drift occurs; it doesn't
guarantee backend's tidy result stays correct.

### 2. `deps-sync` CI job (detection)

The `deps-sync` job in `.github/workflows/ci.yml` runs
`./scripts/deps-sync.sh --check` on every pull request and on every push to
`main` or to a `renovate/**` branch.

The `renovate/**` push trigger is deliberate. `renovate.json5` sets
`automergeType: 'branch'` with `platformAutomerge`, so on the happy path
Renovate can merge a branch without ever opening a PR — and with no PR there
is no `pull_request` event, so the check would never gate the very updates
it was built for. Triggering on the branch push closes that gap regardless
of whether a PR exists. The cost is that when Renovate *does* open a PR,
both events fire and CI runs twice on that branch; Renovate is scheduled
weekly, so that is cheap.

It turns
what would otherwise be a cryptic `npm ci` or `go test` failure buried
inside an unrelated job into a named target ("DRIFT: backend/go.mod") plus
the exact command to fix it.

It also closes two coverage holes that predate it: the `infra` CI job is
currently commented out in `ci.yml` (heavy load / timeout issues), and
`tools/gophmem` appears nowhere else in CI at all — so neither manifest was
being checked by anything before this job existed.

### 3. `renovate-autofix` workflow (repair)

`.github/workflows/renovate-autofix.yml` runs `./scripts/deps-sync.sh --fix`
on every push to a `renovate/**` branch and, if it changed anything, commits
and pushes the fix straight back onto that branch.

## `scripts/deps-sync.sh` usage

```bash
./scripts/deps-sync.sh --check   # non-zero exit on drift, prints remediation
./scripts/deps-sync.sh --fix     # applies fixes; non-zero exit only if a fix command failed
```

Five targets, checked/fixed in this order:

1. `core/go.mod`
2. `backend/go.mod`
3. `tools/gophmem/go.mod`
4. `frontend/package-lock.json`
5. `infra/package-lock.json`

The Go order is fixed — core before backend before tools/gophmem — because
`backend/go.mod`'s `replace ../core` means backend's tidy result depends on
core already being tidy. Tidying in the other order would tidy backend
against a stale core and reproduce the exact drift this script exists to
catch.

**`--check` and `--fix` use different npm commands, on purpose.** `--check`
runs `npm ci --dry-run`, which validates the existing lockfile against
`package.json` and never writes. `--fix` runs
`npm install --package-lock-only`. This isn't an oversight: implementing
`--check` as `npm install --package-lock-only` followed by
`git diff --exit-code` would re-resolve transitive dependencies (picking up
newer semver-compatible versions nobody asked for) and produce
false-positive drift on PRs that never touched this project's dependencies,
turning CI red for unrelated changes. `npm ci --dry-run` only validates
against the lockfile that's already there.

`--check` also distinguishes real drift from other kinds of failure.
`go mod tidy -diff` and `npm ci --dry-run` both exit non-zero for reasons
that have nothing to do with manifest drift — network down, a missing
toolchain, an unresolvable import, a version that doesn't exist. The script
only reports `DRIFT` when the output matches the drift-specific signature (a
unified diff starting with `diff ` for Go; `npm error code EUSAGE` — or the
older `npm ERR! code EUSAGE` — for npm) and reports `ERROR` for everything
else, so a flaky registry doesn't get misreported as a manifest problem.

Set `DEPS_SYNC_NO_MISE=1` to skip `mise x` and use whatever `go` / `node`
is already on `PATH`. This is how CI invokes it: GitHub Actions runners
don't have `mise`, and `actions/setup-go` / `actions/setup-node` already put
the right toolchain versions directly on `PATH`.

**Non-goal:** `scripts/deps-sync.sh` is intentionally *not* wired into
`scripts/check.sh`. `--check` needs network access (both `go mod tidy -diff`
and `npm ci --dry-run` reach out to their registries), and in practice this
drift is essentially only ever introduced by Renovate — keeping the
mandatory local `check.sh` fast and network-free matters more than catching
this specific class locally.

## `RENOVATE_AUTOFIX_TOKEN` setup

`renovate-autofix.yml` pushes its fix commit using the repository secret
`RENOVATE_AUTOFIX_TOKEN` if it's set. To create one:

- **Fine-grained PAT** scoped to this repository only, with repository
  permission **Contents: Read and write**. Or:
- A **GitHub App installation token** for an app installed on this
  repository with the same Contents permission.

Add it under **Settings → Secrets and variables → Actions → New repository
secret**, name `RENOVATE_AUTOFIX_TOKEN`.

The secret changes what happens after the autofix commit is pushed, not
whether the fix itself is applied:

- **Secret present** — the checkout step authenticates as that
  identity, so the resulting push fires `pull_request: synchronize`. CI
  re-runs automatically on the fixed commit. Fully hands-off.
- **Secret absent** — the workflow falls back to the built-in
  `secrets.GITHUB_TOKEN`. The tree is still fixed and the commit still gets
  pushed, but GitHub deliberately does not re-trigger other workflows
  (including this repo's CI) for pushes authored by `GITHUB_TOKEN`, to
  prevent trivial infinite-workflow loops. The PR will show the autofix
  commit, but its checks need a manual "Re-run" or a Renovate rebase to
  pick it up.

**The secret being unset is a supported configuration, not a broken one.**
Every step in `renovate-autofix.yml` works with it undefined — the `||`
fallback in the checkout `token:` input evaluates to `secrets.GITHUB_TOKEN`
whenever `secrets.RENOVATE_AUTOFIX_TOKEN` is unset, since an unset secret is
an empty string, which is falsy in GitHub Actions expression syntax.

**Troubleshooting a 403 on push:** this repository's default workflow
permission is currently read-only (`default_workflow_permissions: "read"`).
`renovate-autofix.yml` opts into `permissions: contents: write` explicitly
at the workflow level, which is the documented way to get write access for
a single workflow without loosening the repo-wide default — but if the
first autofix run fails to push with a 403, check
**Settings → Actions → General → Workflow permissions** first; it's the
most common cause.

## Manual runbook

For when autofix doesn't apply — the fix needs judgment, or the branch
predates this guard:

1. Identify the failure class: `gh pr checks <PR>` to find the failing run,
   then `gh run view <run-id> --log-failed` to see the actual error.
2. **Fetch before you fix.** Renovate force-pushes its branches while you
   may be working on them. Before committing a fix, always
   `git fetch origin <branch>` and reset your local branch onto the current
   tip rather than rebasing your commit onto it. The manifests you're about
   to touch are exactly the ones Renovate's push just regenerated, so a
   rebase conflicts on those same files most of the time — resetting and
   re-running the fix is simpler than resolving that conflict by hand.
3. Run `./scripts/deps-sync.sh --fix`, then `./scripts/check.sh`, then
   commit with a `fix(deps): ...` message.
