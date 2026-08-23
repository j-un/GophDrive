#!/usr/bin/env bash
# Install gophmem CLI + register the gophdrive-memory Skill globally
# (Claude Code and Cursor IDE + CLI/`agent`).
#
# Result:
#   ~/.local/bin/gophmem                               (built from this directory)
#   ~/.claude/skills/gophdrive-memory/SKILL.md   →  symlink into this repo
#   ~/.cursor/skills/gophdrive-memory/SKILL.md   →  same source (Cursor-native)
#
# Re-running is safe: rebuilds the binary, refreshes both symlinks (`ln -snf`).
# Skill pick-up requires a new Claude Code session and a new Cursor CLI/`agent`
# session (Cursor loads skills at process start; `/skills` can confirm).

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
PLUGIN_SKILL_SRC="$SCRIPT_DIR/plugin/skills/gophdrive-memory/SKILL.md"
BIN_DIR="$HOME/.local/bin"
SKILL_DIR="$HOME/.claude/skills/gophdrive-memory"
SKILL_LINK="$SKILL_DIR/SKILL.md"
LEGACY_FLAT_SKILL="$HOME/.claude/skills/gophdrive-memory.md"
CURSOR_SKILL_DIR="$HOME/.cursor/skills/gophdrive-memory"
CURSOR_SKILL_LINK="$CURSOR_SKILL_DIR/SKILL.md"
CURSOR_LEGACY_FLAT_SKILL="$HOME/.cursor/skills/gophdrive-memory.md"

bold() { printf '\033[1m%s\033[0m\n' "$*"; }
warn() { printf '\033[33m! %s\033[0m\n' "$*"; }
ok()   { printf '\033[32m✓ %s\033[0m\n' "$*"; }
info() { printf '  %s\n' "$*"; }

if [[ ! -f "$PLUGIN_SKILL_SRC" ]]; then
  echo "error: SKILL.md source not found at $PLUGIN_SKILL_SRC" >&2
  exit 1
fi

# ---- 1. build CLI ----
bold "Building gophmem ..."
mkdir -p "$BIN_DIR"
if ! command -v mise >/dev/null 2>&1; then
  echo "error: mise is required (see CLAUDE.md → Common Commands). Install via https://mise.jdx.dev/ and re-run." >&2
  exit 1
fi
( cd "$SCRIPT_DIR" && mise x go -- go build -o "$BIN_DIR/gophmem" . )
ok "Installed $BIN_DIR/gophmem"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) warn "$BIN_DIR is not in PATH. Add it to your shell rc:"
     info "  export PATH=\"\$HOME/.local/bin:\$PATH\""
     ;;
esac

# ---- 2. symlink Skill ----
bold "Linking Skill into ~/.claude/skills/ ..."
mkdir -p "$SKILL_DIR"
ln -snf "$PLUGIN_SKILL_SRC" "$SKILL_LINK"
ok "Linked $SKILL_LINK"
info "    → $(readlink "$SKILL_LINK")"

bold "Linking Skill into ~/.cursor/skills/ ..."
mkdir -p "$CURSOR_SKILL_DIR"
ln -snf "$PLUGIN_SKILL_SRC" "$CURSOR_SKILL_LINK"
ok "Linked $CURSOR_SKILL_LINK"
info "    → $(readlink "$CURSOR_SKILL_LINK")"

# ---- 3. legacy flat file detection ----
# Detect both real files and symlinks at the legacy path — either shadows the
# new folder-format Skill of the same name.
if [[ -e "$LEGACY_FLAT_SKILL" || -L "$LEGACY_FLAT_SKILL" ]]; then
  echo
  warn "Legacy flat-file Skill detected:"
  info "  $LEGACY_FLAT_SKILL"
  info "It may shadow or conflict with the new folder-format Skill. Remove it manually:"
  info "  rm \"$LEGACY_FLAT_SKILL\""
fi
if [[ -e "$CURSOR_LEGACY_FLAT_SKILL" || -L "$CURSOR_LEGACY_FLAT_SKILL" ]]; then
  echo
  warn "Legacy flat-file Skill detected:"
  info "  $CURSOR_LEGACY_FLAT_SKILL"
  info "It may shadow or conflict with the new folder-format Skill. Remove it manually:"
  info "  rm \"$CURSOR_LEGACY_FLAT_SKILL\""
fi

# ---- 4. env / config check ----
# Check both the env var and the config file (env takes priority, then file).
echo
_config_file="${GOPHMEM_CONFIG_DIR:-${XDG_CONFIG_HOME:-$HOME/.config}}/gophmem/config"
if [[ -n "${GOPHMEM_API_KEY:-}" ]]; then
  ok "GOPHMEM_API_KEY is set (env)."
elif [[ -f "$_config_file" ]] && grep -q "^GOPHMEM_API_KEY=." "$_config_file"; then
  ok "GOPHMEM_API_KEY is set (config file: $_config_file)."
else
  warn "GOPHMEM_API_KEY is not set."
  info "Issue a key from GophDrive Web UI → Settings → API Keys → Issue Key, then:"
  info ""
  info "  Option A — save to config file (recommended, 0600 permissions):"
  info "    gophmem config set --base-url https://<your-cloudfront-domain>/api   # production only"
  info "    gophmem config set --api-key <the plaintext key you just issued>"
  info ""
  info "  Option B — export in shell rc (~/.zshrc or ~/.bashrc):"
  info "    export GOPHMEM_BASE_URL=https://<your-cloudfront-domain>/api   # production only"
  info "    export GOPHMEM_API_KEY=<the plaintext key you just issued>"
  info ""
  info "For local DEV_MODE you can omit GOPHMEM_BASE_URL (defaults to http://localhost:8080)."
  info "Run 'gophmem config show' to verify the resolved values and their source."
fi

# ---- 5. next steps ----
echo
bold "Next steps"
info "1. Open a new shell so PATH / env vars are picked up."
info "2. Run: gophmem --help"
info "3. Start a new Claude Code session and a new Cursor CLI/agent session"
info "   so the gophdrive-memory Skill is loaded (Cursor loads skills at process"
info "   start; run /skills to confirm)."
