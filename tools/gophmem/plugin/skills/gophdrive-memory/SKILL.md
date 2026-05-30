---
name: gophdrive-memory
description: Use this skill when the user wants to (a) save / record a design decision or reasoning ("保存して" "記録して" "覚えて" "decision を残す"), (b) recall a past decision or rationale ("なぜ前回 X にしたか?" "前回どう決めた?" "過去に同じ判断を見たか?"), (c) log an incident timeline or research findings ("インシデント記録" "incident:" "research:" "調査ログ"), (d) consult / contribute to cross-project, cross-device long-term knowledge that should outlive this conversation, or (e) set up the gophmem CLI itself ("gophmem セットアップ" "GOPHMEM_API_KEY 未設定" "API キー発行"). The skill writes to and queries the user's GophDrive Vault via the gophmem CLI. Prefer the auto-memory at ~/.claude/projects/.../memory/ for short-lived per-project state; prefer this skill for durable cross-cutting "why" notes that benefit from GophDrive's Web UI, graph view, and multi-device sync.
version: 1.0.0
---

# GophDrive External Memory Skill

Use GophDrive as Claude Code's external memory. Record and recall "why" knowledge that cannot be reconstructed from code or git history: design rationale, incident timelines, rejected alternatives, constraints.

## Decide first: this skill vs auto-memory

| Target | What belongs here |
|--------|-------------------|
| `~/.claude/projects/.../memory/` (auto-memory) | Short-lived, project-local, conversation-context aids |
| GophDrive (this skill) | **Cross-cutting, durable, visualizable, multi-device** knowledge |

Write to GophDrive: design rationale, incident timelines, cross-project lessons.
Write to auto-memory: notes that only matter for this conversation.

When the user asks a question that maps to durable knowledge (typical phrasings: 「なぜ前回 X にしたか?」「前回どう決めた?」「過去に同じ判断を見たか?」), **search GophDrive before answering**.

## Tools

```
gophmem write <title> [--tags a,b] [--stdin]   # Create a note in the AI Memory folder
gophmem append <id|title>                       # Append stdin to an existing note
gophmem read <id>                               # Print note content and metadata
gophmem search <query> [--tag t]                # Search the whole Vault
gophmem list                                    # List the AI Memory folder
gophmem tags                                    # List all tags with counts
```

Environment variables:

- `GOPHMEM_BASE_URL`
  - Production: `https://<your-cloudfront-domain>/api`
  - Local dev: unset is fine (defaults to `http://localhost:8080`; Next.js is a static export so `:3000` has no API proxy)
- `GOPHMEM_API_KEY`: issue from the GophDrive Web UI under **Settings → API Keys → Issue Key** (plaintext is shown only once)

Full setup walkthrough: `docs/agent-memory-setup.md` in the GophDrive repository.

## When to record

Record knowledge that cannot be recovered from code or `git log`:

- **Design decisions** — why this architecture was chosen, which alternatives were rejected and why
- **Incidents / open issues** — what happened, root cause, mitigation, residual risk
- **Constraints / preconditions** — external factors (legal, business, performance) that pinned a choice
- **Experiments** — approaches that were tried and dropped, with the result
- **Operational procedures** — manual e2e verification steps, environment-specific setup

Do not record: implementation detail that the code already shows, history that `git log` already tracks, anything already documented in `CLAUDE.md`.

## Note conventions

### Naming

| Kind | Pattern | Example |
|------|---------|---------|
| Decision | `decision: <subject>` | `decision: 認証方式の選択` |
| Daily log | `log/YYYY-MM-DD` | `log/2026-05-31` |
| Incident | `incident: <subject>` | `incident: stale token bug` |
| Research | `research: <subject>` | `research: ベクトル検索比較` |
| How-to | `howto: <subject>` | `howto: agent key ローテ手順` |

Subjects may be in Japanese or English; pick whichever the user typically uses for the topic.

### Frontmatter (required)

```yaml
---
tags:
  - <controlled-tag>   # Choose from the vocabulary below
---
```

### Body structure

```markdown
## Background

(Why this decision was needed)

## Decision

(What was chosen)

## Rejected alternatives

(Options that were compared and dropped, with reasons)

## Follow-ups / caveats

(What a future reader should know before changing this)

## Related

[[Title of a related note]]
```

Link related notes with `[[Title]]` wikilinks.

### Controlled tag vocabulary

```
decision      # Design decisions, architecture choices
incident      # Bugs and incident timelines
auth          # Authentication / authorization
storage       # DynamoDB / S3 design
infra         # CDK / Lambda / CloudFront
frontend      # Next.js / UI
search        # Search quality
agent         # AI agent–related
security      # Security
ops           # Operations / deploy procedures
research      # Comparative investigations
```

## Recording

### Create a new note

```bash
cat <<'EOF' | gophmem write "decision: 認証方式の選択" --tags decision,auth --stdin
## Background
...
EOF
```

### Append to an existing note

```bash
echo "## 2026-05-31 追記\n\n<addition>" | gophmem append "decision: 認証方式の選択"
```

Append a session note to today's daily log:

```bash
echo "## $(date +%H:%M) <subject>\n\n<body>" | gophmem append "log/$(date +%Y-%m-%d)"
```

## Recall

### Search

```bash
gophmem search "認証"             # Keyword search across the whole Vault
gophmem search --tag decision     # Tag filter only (omit the query when empty)
gophmem list                      # List the AI Memory folder
```

### Follow related notes

1. `gophmem read <id>` to see the body
2. Use `[[wikilink]]` references as hints — search them with `gophmem search`
3. Use the GophDrive Web UI's graph view to inspect backlinks visually

### When to consult

When the user asks a "why did we previously choose X?" question (typical phrasings: 「なぜ前回 X にしたか?」「前回どう決めた?」), **always** run a search before answering:

```bash
gophmem search "<keyword>"
```

## Troubleshooting

### `gophmem: command not found` / skill not discovered

Setup has not run. From the GophDrive repository:

```bash
cd /path/to/GophDrive
bash tools/gophmem/install.sh
```

This builds the CLI → installs it to `~/.local/bin/gophmem` → symlinks `~/.claude/skills/gophdrive-memory/SKILL.md` into the repo. **A new Claude Code session is required** for the skill to be loaded.

### `gophmem write` returns 401 (`GOPHMEM_API_KEY` unset or invalid)

1. Open the GophDrive Web UI → **Settings → API Keys → Issue Key** (plaintext is shown only once)
2. Add to `~/.zshrc` or `~/.bashrc`:
  ```bash
  export GOPHMEM_BASE_URL=https://<your-cloudfront-domain>/api   # production only
  export GOPHMEM_API_KEY=<the plaintext key>
  ```
3. Open a new shell and retry

Details and the local-dev case: `docs/agent-memory-setup.md`.

### `gophmem write` returns 403

You are signed in as a demo account. Sign in with your Google account, or issue an API key under that account.

### Orphan notes (not visible in the UI)

A note whose parent folder ID no longer exists is skipped by `ListFiles` and becomes invisible in the UI.

**Confirm**: if you know the note ID, `GET /api/notes/<id>` returns it directly.

**Recover (re-attach to a valid parent)**:

```bash
curl -X PATCH \
  -H "Authorization: Bearer $GOPHMEM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"parentId": "<valid folder id>"}' \
  "$GOPHMEM_BASE_URL/notes/<note id>"
```

**Or delete it**:

```bash
curl -X DELETE \
  -H "Authorization: Bearer $GOPHMEM_API_KEY" \
  "$GOPHMEM_BASE_URL/notes/<note id>"
```

The root folder ID (`base_folder_id`) can be read from the JWT's `base_folder_id` claim.
