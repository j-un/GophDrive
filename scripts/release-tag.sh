#!/usr/bin/env bash
#
# Create an annotated release tag and push it to origin.
#
# Usage:
#   scripts/release-tag.sh <semver>
#
# Example:
#   scripts/release-tag.sh v0.1.0
#
# Preconditions:
#   - Argument must be a semantic version prefixed with 'v' (e.g. v1.2.3).
#   - Working tree must be clean (no staged, unstaged, or untracked changes).
#   - Current HEAD must be exactly origin/main (no divergence).

set -euo pipefail

if [ "$#" -ne 1 ]; then
  echo "Usage: $0 <semver>" >&2
  echo "Example: $0 v0.1.0" >&2
  exit 1
fi

VERSION="$1"

# Validate semantic version format: vMAJOR.MINOR.PATCH with optional pre-release/build metadata.
SEMVER_REGEX='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'
if ! [[ "$VERSION" =~ $SEMVER_REGEX ]]; then
  echo "ERROR: '$VERSION' is not a valid semantic version (expected e.g. v1.2.3)." >&2
  exit 1
fi

# Ensure we are inside a git repository.
if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "ERROR: not inside a git repository." >&2
  exit 1
fi

echo "--- Fetching origin... ---"
git fetch origin --tags --prune

# Ensure the tag does not already exist locally or remotely.
if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null; then
  echo "ERROR: tag '$VERSION' already exists locally." >&2
  exit 1
fi
if git ls-remote --exit-code --tags origin "refs/tags/$VERSION" >/dev/null 2>&1; then
  echo "ERROR: tag '$VERSION' already exists on origin." >&2
  exit 1
fi

# Working tree must be clean (no staged, unstaged, or untracked changes).
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: working tree is not clean. Commit or stash changes first." >&2
  git status --short >&2
  exit 1
fi

# HEAD must equal origin/main exactly.
LOCAL_SHA="$(git rev-parse HEAD)"
REMOTE_SHA="$(git rev-parse origin/main)"
if [ "$LOCAL_SHA" != "$REMOTE_SHA" ]; then
  echo "ERROR: HEAD does not match origin/main." >&2
  echo "  HEAD        : $LOCAL_SHA" >&2
  echo "  origin/main : $REMOTE_SHA" >&2
  echo "Check out main and pull the latest changes before tagging." >&2
  exit 1
fi

echo "--- Creating annotated tag $VERSION at $LOCAL_SHA ---"
git tag -a "$VERSION" -m "Release $VERSION"

echo "--- Pushing tag $VERSION to origin ---"
git push origin "refs/tags/$VERSION"

echo "--- Done. Tag $VERSION pushed. ---"
