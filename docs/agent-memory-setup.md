# AI Agent External Memory Setup

How to use GophDrive as Claude Code's external memory.

## Overview

The `gophmem` CLI hits the GophDrive REST API, and Claude Code follows the skill at `~/.claude/skills/gophdrive-memory/SKILL.md` to record and recall notes. The agent operates under your Google identity (same `sub`) and writes to the `AI Memory` folder in your Vault. The skill's source-of-truth is `tools/gophmem/plugin/skills/gophdrive-memory/SKILL.md` in this repository; `install.sh` symlinks it into `~/.claude/skills/`.

## Prerequisites

- GophDrive is deployed to production (`./scripts/deploy-aws.sh` has run)
- You can sign into GophDrive with Google
- `mise` is installed (used to build the CLI against the Go version pinned in `.tool-versions`)

---

## 1. Install CLI and skill in one shot

From the repository root:

```bash
bash tools/gophmem/install.sh
```

The script does all of the following:

- Builds `tools/gophmem` and places the binary at `~/.local/bin/gophmem`
- Symlinks the `gophdrive-memory` Claude Code skill into `~/.claude/skills/gophdrive-memory/SKILL.md` (source: `tools/gophmem/plugin/skills/gophdrive-memory/SKILL.md`)
- Warns if a legacy flat-file skill at `~/.claude/skills/gophdrive-memory.md` is still present
- Warns when `GOPHMEM_API_KEY` is unset and shows the export commands you need

If `~/.local/bin` is not on your `PATH`, the script warns; add `export PATH="$HOME/.local/bin:$PATH"` to your shell rc.

Re-run the script after `git pull` updates `tools/gophmem/`. Only the binary is rebuilt — the skill is a symlink and reflects repo changes immediately.

---

## 2. Issue an API key

1. Open GophDrive in the browser and sign in with Google
2. Open **Settings** from the top-right menu
3. Click **Issue Key** under the **API Keys** section
4. Copy the plaintext key shown in the dialog — it is shown only once

---

## 3. Set environment variables

```bash
# Append to ~/.zshrc or ~/.bashrc
export GOPHMEM_BASE_URL=https://<your-cloudfront-domain>/api
export GOPHMEM_API_KEY=<the plaintext key from step 2>
```

---

## 4. Smoke test

```bash
# Create the AI Memory folder and write the first note
gophmem write "howto: agent memory setup complete" --tags agent,ops

# List
gophmem list

# Search
gophmem search "setup"
```

The note should appear in the `AI Memory` folder in the Web UI.

---

## Local development

Add the following to your `.env` (see `.env.example`):

```bash
API_KEY_HASHES_TABLE=APIKeyHashes
```

For local dev, `gophmem`'s default URL is `http://localhost:8080`, so `GOPHMEM_BASE_URL` can be omitted. (Next.js is a static export, so port `:3000` has no API proxy — `gophmem` talks to the backend on `:8080` directly.)

Issue the API key from the local GophDrive UI the same way as step 2.

---

## Key rotation and revocation

Both are available immediately under **Settings → API Keys**.

- **Regenerate Key**: revokes the old key and issues a new one atomically in DynamoDB.
- **Revoke**: invalidates the key. To use the integration again, issue a new key.

> **Difference from the legacy SSM scheme**: the old scheme kept the old key valid until Lambda cold start. The current scheme is a DynamoDB lookup, so revocation is **immediate**.

---

## Skill installation

Step 1 (`bash tools/gophmem/install.sh`) symlinks `~/.claude/skills/gophdrive-memory/SKILL.md` to `tools/gophmem/plugin/skills/gophdrive-memory/SKILL.md` in this repo. No extra action is required.

A **new Claude Code session** is required for the skill to be picked up. The repo-managed file `tools/gophmem/plugin/skills/gophdrive-memory/SKILL.md` is the single source of truth.

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| `gophmem write` → 401 | `GOPHMEM_API_KEY` is wrong | Settings → API Keys → Regenerate, re-copy the plaintext |
| `gophmem write` → 401 (local) | No key was issued, or it was issued in a different environment | Issue a key from the local GophDrive UI and re-export |
| `gophmem write` → 403 | Signed in as a demo account | Switch to your Google account |
| `AI Memory` folder not found | First-time folder creation failed | Re-run `gophmem list`. As a last resort, delete `~/.cache/gophmem/folders.json` and retry |
