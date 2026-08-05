#!/usr/bin/env bash
# Single source of truth for "are the dependency manifests in sync?" and "fix them".
# Shared by three consumers: the CI drift-detection job, a Renovate auto-fix GitHub
# Actions workflow, and humans running it locally.
#
# Usage:
#   scripts/deps-sync.sh --check   # non-zero exit on drift, prints remediation
#   scripts/deps-sync.sh --fix     # applies fixes; non-zero exit only if a fix command failed
#
# Background: backend/go.mod has `replace github.com/jun/gophdrive/core => ../core`.
# Renovate runs `go mod tidy` per-directory with `replace` statements massaged away, so
# when core/go.mod's direct deps bump, backend/go.mod's `// indirect` lines go stale and
# `go test ./...` fails with "go: updates to go.mod needed". core must therefore be tidied
# before backend. Separately, Renovate's npm lockfile artifact update sometimes fails,
# leaving package.json updated but package-lock.json stale, so `npm ci` fails with
# "EUSAGE / lock file's react@19.2.7 does not satisfy react@19.2.8".
set -uo pipefail # deliberately no -e: --check must aggregate across every target, not abort on the first drift

cd "$(dirname "$0")/.."

GREEN='\033[0;32m'; NC='\033[0m'
section() { echo -e "\n${GREEN}==> $* <==${NC}"; }

usage() {
  cat >&2 <<'EOF'
Usage: scripts/deps-sync.sh --check | --fix

  --check   Report drifted dependency manifests (go.mod / package-lock.json). Non-zero exit on drift.
  --fix     Apply fixes (go mod tidy / npm install --package-lock-only). Non-zero exit only
            if a fix command itself failed; a clean run exits 0 whether or not it changed files.
EOF
}

if [ "$#" -ne 1 ]; then
  usage
  exit 1
fi
MODE="$1"
case "$MODE" in
--check | --fix) ;;
*)
  usage
  exit 1
  ;;
esac

# Toolchain resolution: locally the repo drives toolchains through mise (see
# scripts/check.sh). GitHub Actions runners have no mise; actions/setup-go and
# actions/setup-node put the right binaries directly on PATH instead. Support both,
# with an explicit escape hatch for CI.
USE_MISE=1
if [ "${DEPS_SYNC_NO_MISE:-}" = "1" ] || ! command -v mise >/dev/null 2>&1; then
  USE_MISE=0
fi

run_go() {
  if [ "$USE_MISE" = "1" ]; then
    mise x go -- bash -c "$1"
  else
    bash -c "$1"
  fi
}

run_node() {
  if [ "$USE_MISE" = "1" ]; then
    mise x node -- bash -c "$1"
  else
    bash -c "$1"
  fi
}

DRIFTED=()
ERRORED=()
FAILED=()

go_check() {
  local dir="$1"
  section "Go: ${dir}/go.mod"
  # `go mod tidy -diff` (Go 1.23+) writes nothing and exits non-zero when a diff is needed.
  # It also exits non-zero on a genuine error (missing toolchain, network down, unresolvable
  # import). Only a real diff prints a unified diff (starting with "diff ") to stdout, so
  # stdout's first line is the signal. Note that stderr is NOT a signal either way: a genuine
  # drift also writes to stderr (e.g. "go: found ... in ..."). Capture the streams separately
  # so we can key off stdout alone instead of reporting every failure as drift.
  local out err status errfile
  errfile="$(mktemp)"
  out="$(run_go "cd '${dir}' && go mod tidy -diff" 2>"$errfile")"
  status=$?
  err="$(cat "$errfile")"
  rm -f "$errfile"
  [ -n "$out" ] && echo "$out"
  [ -n "$err" ] && echo "$err" >&2

  # Matched with `case` rather than a `printf | head | grep` pipeline: under `pipefail` a
  # short-circuiting reader can SIGPIPE the writer on a large diff and flip the result.
  if [ "$status" -eq 0 ]; then
    echo "OK: ${dir}/go.mod is tidy."
  elif case "$out" in "diff "*) true ;; *) false ;; esac; then
    echo "DRIFT: ${dir}/go.mod is not tidy."
    echo "  fix: ./scripts/deps-sync.sh --fix   (or: cd ${dir} && go mod tidy)"
    DRIFTED+=("${dir}/go.mod")
  else
    echo "ERROR: go mod tidy -diff failed for ${dir} for a reason other than drift (see output above)."
    ERRORED+=("${dir} (go mod tidy -diff)")
  fi
}

go_fix() {
  local dir="$1"
  section "Go: ${dir}/go.mod"
  if ! run_go "cd '${dir}' && go mod tidy"; then
    echo "ERROR: go mod tidy failed for ${dir} (see output above)."
    FAILED+=("${dir} (go mod tidy)")
  fi
}

npm_check() {
  local dir="$1"
  section "npm: ${dir}/package-lock.json"
  # `npm ci --dry-run` is exactly the EUSAGE condition CI hits when package-lock.json is
  # stale relative to package.json.
  #
  # This is deliberately NOT implemented as `npm install --package-lock-only` followed by
  # `git diff --exit-code`: that re-resolves transitive dependencies (picking up newer
  # semver-compatible versions nobody asked for) and would produce false-positive drift on
  # PRs that never touched this project's dependencies, turning CI red for unrelated
  # changes. `npm ci --dry-run` only validates the existing lockfile against package.json
  # and never writes. Do not "simplify" this back to the diff form.
  #
  # `npm ci --dry-run` also exits non-zero for reasons unrelated to lockfile drift (registry
  # unreachable, a version that doesn't exist, etc). Only the lockfile-out-of-sync case
  # prints "npm error code EUSAGE" (older npm: "npm ERR! code EUSAGE"), so key off that
  # instead of treating every non-zero exit as drift.
  local out err status errfile
  errfile="$(mktemp)"
  out="$(run_node "cd '${dir}' && npm ci --dry-run" 2>"$errfile")"
  status=$?
  err="$(cat "$errfile")"
  rm -f "$errfile"
  [ -n "$out" ] && echo "$out"
  [ -n "$err" ] && echo "$err" >&2

  # `case` rather than a `printf | grep` pipeline, for the same pipefail/SIGPIPE reason as
  # go_check above.
  if [ "$status" -eq 0 ]; then
    echo "OK: ${dir}/package-lock.json is in sync."
  elif case "${out}
${err}" in *'npm error code EUSAGE'* | *'npm ERR! code EUSAGE'*) true ;; *) false ;; esac; then
    echo "DRIFT: ${dir}/package-lock.json is out of sync with package.json."
    echo "  fix: ./scripts/deps-sync.sh --fix   (or: cd ${dir} && npm install --package-lock-only)"
    DRIFTED+=("${dir}/package-lock.json")
  else
    echo "ERROR: npm ci --dry-run failed for ${dir} for a reason other than lockfile drift (see output above)."
    ERRORED+=("${dir} (npm ci --dry-run)")
  fi
}

npm_fix() {
  local dir="$1"
  section "npm: ${dir}/package-lock.json"
  if ! run_node "cd '${dir}' && npm install --package-lock-only --ignore-scripts --no-audit --no-fund"; then
    echo "ERROR: npm install --package-lock-only failed for ${dir} (see output above)."
    FAILED+=("${dir} (npm install --package-lock-only)")
  fi
}

if [ "$MODE" = "--check" ]; then
  # Order mirrors --fix for readability, but it carries no meaning here: `go mod tidy -diff`
  # is read-only, so each module is diffed against the core/go.mod already on disk regardless
  # of sequence. Only --fix genuinely depends on core being tidied before backend.
  go_check core
  go_check backend
  go_check tools/gophmem
  npm_check frontend
  npm_check infra

  if [ "${#DRIFTED[@]}" -gt 0 ] || [ "${#ERRORED[@]}" -gt 0 ]; then
    section "Summary"
    # Guard each loop on its own length: under bash 3.2 (macOS's default /bin/bash),
    # `"${arr[@]}"` on a zero-element array raises "unbound variable" with `set -u`, even
    # though `${#arr[@]}` on the same empty array is fine. DRIFTED and ERRORED can each be
    # empty while the other isn't, so both need their own guard here.
    if [ "${#DRIFTED[@]}" -gt 0 ]; then
      for d in "${DRIFTED[@]}"; do
        echo "  DRIFT: ${d}"
      done
    fi
    if [ "${#ERRORED[@]}" -gt 0 ]; then
      for e in "${ERRORED[@]}"; do
        echo "  ERROR: ${e}"
      done
    fi
    exit 1
  fi

  section "All dependency manifests are in sync."
  exit 0
fi

# --fix
# Each *_fix step below records failure instead of aborting, so one failing target (network
# blip, an ERESOLVE-style resolution error) doesn't hide the status of the others, and so we
# never fall through to the unconditional success this script used to have — a silent "(no
# changes)" + exit 0 on failure would make the Renovate auto-fix workflow conclude "no
# drift" and do nothing, which is exactly the silent-failure mode this script exists to kill.
go_fix core
go_fix backend
go_fix tools/gophmem
npm_fix frontend
npm_fix infra

section "Files changed by --fix"
CHANGED="$(git status --porcelain -- \
  core/go.mod core/go.sum \
  backend/go.mod backend/go.sum \
  tools/gophmem/go.mod tools/gophmem/go.sum \
  frontend/package-lock.json infra/package-lock.json)"
if [ -n "$CHANGED" ]; then
  echo "$CHANGED"
else
  echo "(no changes)"
fi

if [ "${#FAILED[@]}" -gt 0 ]; then
  section "Summary: --fix failed for these targets"
  for f in "${FAILED[@]}"; do
    echo "  - ${f}"
  done
  exit 1
fi

exit 0
