---
name: gophdrive-memory
description: Use this skill when the user wants to (a) save / record a design decision or reasoning ("保存して" "記録して" "覚えて" "decision を残す"), (b) recall a past decision or rationale ("なぜ前回 X にしたか?" "前回どう決めた?" "過去に同じ判断を見たか?"), (c) log an incident timeline or research findings ("インシデント記録" "incident:" "research:" "調査ログ"), (d) consult / contribute to cross-project, cross-device long-term knowledge that should outlive this conversation, or (e) set up the gophmem CLI itself ("gophmem セットアップ" "GOPHMEM_API_KEY 未設定" "API キー発行"). The skill writes to and queries the user's GophDrive Vault via the gophmem CLI. Prefer the auto-memory at ~/.claude/projects/.../memory/ for short-lived per-project state; prefer this skill for durable cross-cutting "why" notes that benefit from GophDrive's Web UI, graph view, and multi-device sync.
version: 1.1.0
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
gophmem search <query> [--tag t]... [--type t] [--since D] [--until D] [--limit N] [--no-snippet] [--in titles|headings|all]  # Search the whole Vault
gophmem list                                    # List the AI Memory folder
gophmem tags                                    # List all tags with counts
gophmem links <id|title>                        # Outbound wikilinks of a note
gophmem backlinks <id|title>                    # Notes linking to a note
gophmem graph [--center <id|title>] [--depth N] # Adjacency view (default: whole vault)
gophmem unresolved                              # Wikilinks whose target note does not exist
```

**Search flags**: `--since`/`--until` accept `YYYY-MM-DD` (local time; `--until` includes that whole day) or RFC3339; `--tag` is repeatable (AND); `--type` filters by frontmatter `type:`.

**Caution**: `--tags` prepends its own `---\ntags:...\n---` block — when writing your own frontmatter (`type`/`aliases`/`status`), put tags in it and omit `--tags`.

Setup, env vars (`GOPHMEM_BASE_URL`, `GOPHMEM_API_KEY`), and troubleshooting (401/403, missing key, AI Memory folder issues): `docs/agent-memory-setup.md` in the GophDrive repository. Credentials can also be stored in `~/.config/gophmem/config` via `gophmem config set` (0600, env vars take priority).

### Examples

```bash
# Create with type and aliases
cat <<'EOF' | gophmem write "decision: 認証方式の選択" --stdin
---
tags: [decision, auth]
type: decision
aliases:
  - Auth approach choice
status: active
---

## Background
...
EOF

# Append to an existing note (by title or ID)
echo "## 2026-05-31 追記\n\n<addition>" | gophmem append "decision: 認証方式の選択"

# Append to today's daily log
echo "## $(date +%H:%M) <subject>\n\n<body>" | gophmem append "log/$(date +%Y-%m-%d)"

# Search before answering a "why" question
gophmem search "認証"
gophmem search --tag decision --type decision
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

**Frontmatter (tags required; others optional)**:

```yaml
---
tags: [decision, auth]
type: decision        # decision | log | incident | research | howto
status: active        # active | superseded | draft
aliases:
  - 認証方式の決定
---
```

`type` mirrors the title-prefix vocabulary and is filterable via `gophmem search --type`. `aliases` let `[[wikilinks]]` and searches match alternate names (exact titles always win). `status: superseded` marks decisions replaced by newer notes.

**Body sections**: `## Background` / `## Decision` / `## Rejected alternatives` / `## Follow-ups / caveats` / `## Related`. Link related notes with `[[Title]]` wikilinks.

**Controlled tag vocabulary**: `decision`, `incident`, `auth`, `storage`, `infra`, `frontend`, `search`, `agent`, `security`, `ops`, `research`.

## Recall

1. `gophmem search <keyword>` — always run before answering a "why did we previously…" question.
2. `gophmem read <id>` for the full body.
3. Traverse the vault directly:
   - `gophmem backlinks <title>` — see what links to a note.
   - `gophmem graph --center <title> --depth 2` — explore the neighborhood.
   - `gophmem search <q> --type decision --since <date>` — scoped recall by type and date.
   - `gophmem unresolved` — find notes promised via `[[links]]` but never written (candidates for new notes).
