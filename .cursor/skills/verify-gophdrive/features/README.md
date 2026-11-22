# GophDrive verification map

This directory is the maintained source for verifying the user-facing behavior of the GophDrive web app. Read this index, then use the matching feature file as the recipe.

## Baseline preconditions

- `gophdrive-verify.mjs doctor` prints `worth driving: yes` for `http://localhost:5173`.
- The verification Chrome is the one `browser start` launched. Do not attach to the user's Chrome.
- Every recipe begins signed out, then uses **Try Demo Mode**. That mints a new demo user and seeds two notes, shown as `Welcome!` and `ようこそ!`.
- Do not click **Login with Google**. Local dummy OAuth credentials leave the app.
- Drive `http://localhost:5173`, never `http://127.0.0.1:5173`.

Baseline demo login, after `browser start`:

```bash
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser goto http://localhost:5173/
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Try Demo Mode"
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Core Module Active" --timeout 45000
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Try Demo Mode"
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/ --timeout 30000
mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Welcome!"
```

The drive heading is `Home`. Both seeded notes are visible. `browser cookies` includes `session_token` on `localhost`.

## Driving conventions

- Start every recipe from the baseline state unless its preconditions say otherwise.
- Prefer role and accessible name. Placeholder `Search...` and `Folder name...` are exact.
- The button chip says `⌘⌥N`. The harness sends `Alt+Meta+n` (Windows and Linux use `Alt+Ctrl+n`). Plain `⌘N` and `Ctrl+N` are the browser's new window, and the page ignores them. The search shortcut is `⌘K`. The harness sends `Ctrl+k` for that listener. `Meta+k` is the same chord.
- Treat every command as literal.
- Restore nothing by hand. Demo notes expire after 60 minutes. Do not delete other users' rows.
- Do not remove proof artifacts during cleanup.

## Proof and skip reporting

- Capture the user action and the resulting state, not only the final screen.
- UI proof is a `browser snapshot` plus a `browser screenshot` with the GophDrive title and the feature's text visible.
- A saved note is proven by leaving it and opening it again from the drive list.
- An export is proven by a `.zip` file under `evidence/export/`, not by the button returning to rest.
- Record the feature id and the entry point with the artifacts (the snapshot's `url:` line is enough when the path shows which control was used).
- Report an unreachable path with the command that failed and the unmet precondition.
- Do not report a skipped entry point as verified through a different path.

## Feature entry contract

Each feature file starts with an H1 and one paragraph, then exactly these H2s in order: `Sub-features`, `How to get to it (user POV)`, `Driving it with gophdrive-verify`, `Gotchas`.

## Features

- [Demo login](./demo-login.md) signs in from the home page and lands on a private drive with the two seeded notes.
- [Create a note](./create-note.md) creates a note from the New note button and from ⌘⌥N, opens the editor, and reopens that note from the drive list.
- [Search notes](./search.md) finds a seeded note by title and by body, shows an empty query, and leaves search.
- [Edit a note](./edit-note.md) types into the editor, waits for autosave, reopens the note, and opens preview.
- [Export notes](./export-notes.md) downloads a ZIP of every note from Settings.
