---
name: verify-gophdrive
description: Drive the GophDrive web app in a disposable Chrome profile against the local Vite dev server (localhost:5173) to prove demo login, notes, search, edit, and export. Use when a change touches the UI, the session cookie, or a note flow and unit tests are not enough.
---

# Verify GophDrive

GophDrive's primary surface is the Vite + React app at `http://localhost:5173`. The Go API on `:8080` and DynamoDB Local on `:8000` are part of that same local stack. The gophmem CLI is a second surface; its recipe is `.claude/skills/verify/SKILL.md`. This skill does not drive gophmem.

Ports are fixed. Two stacks cannot run side by side. If doctor reports the stack already usable, adopt it: drive a fresh demo user in the verification Chrome, and do not restart or quit overmind. Demo login mints `demo-user-<uuid>` with a 60-minute TTL, so a new demo user does not overwrite another user's notes. Never drive the user's own Chrome profile.

The session cookie is host-only and `FRONTEND_URL` is `http://localhost:5173`. Open that host. `http://127.0.0.1:5173` sets the cookie on a different host and the app bounces back to the signed-out home page.

## Launch

From the repo root. Requires a one-time `./scripts/setup.sh` (mise, npm, DynamoDB tables, `.env`, `core.wasm`, and `air` for the pinned Go) and a running Docker daemon. Launch does not start the daemon. On this machine that is `rdctl start`, then wait until `docker info` succeeds. Do not copy or rewrite `.env`.

`air` is per Go version. After a Go bump, `mise x go -- go install github.com/air-verse/air@v1.67.4` (the same command as `scripts/setup.sh`). If `run/overmind.log` says `air is not provided`, install it and run `launch` again. Overmind stops the whole session when the backend exits, so a missing `air` takes the frontend down with it.

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs launch
```

Launch adopts a stack that is already usable. Otherwise it starts DynamoDB Local only when `:8000` is down, then `overmind start` (Procfile: backend, frontend, wasm). Ready means all of these are true:

- `http://localhost:5173/` is HTML whose title is `GophDrive`
- `http://localhost:8080/auth/user` returns `401` with body `Unauthorized`
- DynamoDB Local accepts a connection on `127.0.0.1:8000`
- `http://localhost:5173/core.wasm` returns `200`

The home page is usable when it shows `Core Module Active` and the link `Try Demo Mode`. A listening port is not enough: wait for that text before clicking demo login.

`launch` prints `ready http://localhost:5173`.

## Doctor

Read-only. Run this first whenever a drive looks wrong.

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs doctor
```

Exit `0` and `worth driving: yes` means the checks in Launch pass and the listeners are this repo's dev stack. Exit `1` means do not drive. If doctor says a port is taken by something that is not GophDrive, stop. This skill does not kill processes it did not start.

## Drive

Start a disposable Chrome, then run the commands in the feature file. The browser stays up across commands. Viewport is 1280×900 so the desktop sidebar is showing.

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser start
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser goto http://localhost:5173/
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Try Demo Mode"
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Search..." --value "Welcome"
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Welcome!"
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/search/results.png
```

`--name` matches the accessible name (`aria-label`, otherwise visible text) as a substring. Hidden controls are skipped. `wait-name` waits for that accessible name and does not click. `--placeholder` is an exact match. `--key` is one of `Enter`, `Escape`, `Meta+n`, `Meta+k`, `Ctrl+n`, `Ctrl+k`, `Alt+Meta+n`, `Alt+Ctrl+n`.

`browser insert --text` types into the open note's CodeMirror field (`.cm-content`) at the caret. A note that just opened has the caret at the start.

`browser cookies` prints cookie names and hosts. It does not print values.

Follow `.cursor/skills/verify-gophdrive/features/README.md`, then the feature file. Drive the entry point that file lists. A pass through a different entry point does not cover the one you skipped.

## Evidence

Write proof under `.cursor/skills/verify-gophdrive/evidence/<feature>/`. This directory is gitignored and cleanup does not delete it.

For each feature capture:

- the user action and the state after it (a snapshot while the editor is open, and another after leaving and coming back)
- `browser snapshot` (url, `h1`, visible buttons, links, and textboxes)
- `browser screenshot` of that same state
- a second user-facing view of anything that was saved (reopen the note from the drive list, or the copied ZIP)

Do not call note APIs yourself, and do not sign a session cookie by hand. Demo login through the home page is the session. The local stack already stands in for AWS (DynamoDB Local, `DEV_MODE=true`). Do not mock the API.

A ZIP export is real once a `.zip` file exists under `evidence/export/`. `run/downloads/` is scratch and cleanup removes it, so copy the file first:

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser downloads --copy-to .cursor/skills/verify-gophdrive/evidence/export
```

## Cleanup

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs cleanup
```

Stops the verification Chrome (the process launched with this skill's profile directory). Quits overmind only when this skill's `launch` started it. Stops the DynamoDB Local container only when this skill started it. Does not delete the DynamoDB volume and does not delete `evidence/`.

Run cleanup after a failed attempt too, so a half-started Chrome or overmind is not left behind. Then run `launch` again.

## Helpers

The harness is one executable script. Invoke it with mise's Node (22+):

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs --help
```

`browser start` refuses a second verification Chrome. Run `cleanup` before starting another. Set `GOPHDRIVE_CHROME` if Google Chrome is not in `/Applications`.
