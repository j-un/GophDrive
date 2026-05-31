---
name: gophdrive-memory
description: Use this skill when the user wants to (a) save / record a design decision or reasoning ("保存して" "記録して" "覚えて" "decision を残す"), (b) recall a past decision or rationale ("なぜ前回 X にしたか?" "前回どう決めた?" "過去に同じ判断を見たか?"), (c) log an incident timeline or research findings ("インシデント記録" "incident:" "research:" "調査ログ"), (d) consult / contribute to cross-project, cross-device long-term knowledge that should outlive this conversation, or (e) set up the gophmem CLI itself ("gophmem セットアップ" "GOPHMEM_API_KEY 未設定" "API キー発行"). The skill writes to and queries the user's GophDrive Vault via the gophmem CLI. Prefer the auto-memory at ~/.claude/projects/.../memory/ for short-lived per-project state; prefer this skill for durable cross-cutting "why" notes that benefit from GophDrive's Web UI, graph view, and multi-device sync.
version: 1.0.0
---

# GophDrive External Memory Skill

Use GophDrive as Claude Code's external memory. Record and recall "why" knowledge that cannot be reconstructed from code or git history: design rationale, incident timelines, rejected alternatives, constraints.

## This skill vs auto-memory

Short-lived, project-local context aids → auto-memory (`~/.claude/projects/.../memory/`). Cross-cutting, durable, multi-device "why" notes → this skill.

When the user asks "why did we previously choose X?" (typical: 「なぜ前回 X にしたか?」「前回どう決めた?」「過去に同じ判断を見たか?」), **search GophDrive before answering**.

## Commands

```
gophmem write <title> [--tags a,b] [--stdin]   # Create note in AI Memory folder
gophmem append <id|title>                       # Append stdin to an existing note
gophmem read <id>                               # Print note content and metadata
gophmem search <query> [--tag t]                # Search the whole Vault
gophmem list                                    # List the AI Memory folder
gophmem tags                                    # List all tags with counts
```

Setup, env vars (`GOPHMEM_BASE_URL`, `GOPHMEM_API_KEY`), and troubleshooting (401/403, missing key, AI Memory folder issues): `docs/agent-memory-setup.md` in the GophDrive repository.

### Examples

```bash
# Create
cat <<'EOF' | gophmem write "decision: 認証方式の選択" --tags decision,auth --stdin
## Background
...
EOF

# Append to an existing note (by title or ID)
echo "## 2026-05-31 追記\n\n<addition>" | gophmem append "decision: 認証方式の選択"

# Append to today's daily log
echo "## $(date +%H:%M) <subject>\n\n<body>" | gophmem append "log/$(date +%Y-%m-%d)"

# Search before answering a "why" question
gophmem search "認証"
gophmem search --tag decision
```

## When to record

Record knowledge that cannot be recovered from code or `git log`:

- **Design decisions** — chosen architecture, rejected alternatives and why
- **Incidents / open issues** — what happened, root cause, mitigation, residual risk
- **Constraints** — external factors (legal, business, performance) that pinned a choice
- **Experiments** — approaches tried and dropped, with the result
- **Operational procedures** — manual e2e verification, environment-specific setup

Do not record: implementation detail the code already shows, history already in `git log`, or anything already in `CLAUDE.md`.

## Note conventions

**Naming**: `decision: <subject>` / `log/YYYY-MM-DD` / `incident: <subject>` / `research: <subject>` / `howto: <subject>`. Subject can be JP or EN — match the user's vocabulary for the topic.

**Frontmatter (required)**:

```yaml
---
tags:
  - <controlled-tag>
---
```

**Body sections**: `## Background` / `## Decision` / `## Rejected alternatives` / `## Follow-ups / caveats` / `## Related`. Link related notes with `[[Title]]` wikilinks.

**Controlled tag vocabulary**: `decision`, `incident`, `auth`, `storage`, `infra`, `frontend`, `search`, `agent`, `security`, `ops`, `research`.

## Recall

1. `gophmem search <keyword>` — always run before answering a "why did we previously…" question.
2. `gophmem read <id>` for the full body.
3. Follow `[[wikilink]]` references with further searches; or inspect backlinks visually in the Web UI's graph view.
