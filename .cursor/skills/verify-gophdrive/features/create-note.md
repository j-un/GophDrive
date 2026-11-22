# Create a note

**+ New note** and ⌘⌥N create a note named with the local date and time, open it in the editor, and show that note again from the drive list. There is no name field.

## Sub-features

- `create-button` creates the note from **+ New note** and opens the editor.
- `create-shortcut` does the same from ⌘⌥N when focus is outside a text field.
- `create-open` shows the timestamp title. The editor body is `# ` and does not contain that timestamp. After `Hello` is inserted, the visible text contains `# Hello`.
- `create-persist` shows the new note in the drive list and opens it again.

## How to get to it (user POV)

- Choose **+ New note** on the drive. The chip reads `⌘⌥N`.
- Press `⌘⌥N` or `Ctrl+Alt+N` while focus is outside a text field.
- The editor opens at `/note/`. The title is the local time `YYYY-MM-DD HH:mm:ss`. The editor body is `# ` and does not contain that timestamp. You do not type a name. Escape does not cancel.

## Driving it with gophdrive-verify

Start from the baseline demo login on `/drive/` with heading `Home`.

The title is not known ahead of time. After the editor opens, run `browser text`. Copy the title that matches `YYYY-MM-DD HH:mm:ss`. A snapshot shows the same title. In the commands below, `<title>` is that copied string. Every other word is literal.

- **Plain ⌘N does not create.** Press ⌘N. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Meta+n`. The address stays on `/drive/` and the heading stays `Home`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Home"`.
- **Open from the button.** Choose **+ New note**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "New note"`. The address contains `/note/`. There is no name field. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/ --timeout 15000`.
- **Read the title.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser text`. Copy the timestamp title. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/create-note/editor.txt` when you want the same text in a file.
- **Editor shows the note.** The visible title is `<title>`. The editor body is `# ` and does not contain `<title>`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "<title>"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "# <title>"`.
- **Caret continues the heading.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser insert --text "Hello"`. The visible text contains `# Hello`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "# Hello"`.
- **Capture the editor.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/create-note/editor.png`. The picture shows `# Hello`.
- **Return to the list.** Choose **Back to Drive**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Back to Drive"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/`. The list includes `<title>` and still includes `Welcome!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "<title>"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Welcome!"`.
- **Reopen.** Choose the link whose name includes `<title>`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "<title>"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/`. The title is `<title>` again. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "<title>"`.
- **Proof.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/create-note/reopened.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/create-note/reopened.png`. Both artifacts show `<title>` on `/note/`.
- **Open from the keyboard.** Return to the drive. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Back to Drive"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/`. Focus must be outside a text field. Press ⌘⌥N. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Alt+Meta+n`. The address contains `/note/` and the visible title matches `YYYY-MM-DD HH:mm:ss`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/ --timeout 15000` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser text`.

## Gotchas

- The accessible name of **+ New note** includes the `⌘⌥N` chip. Match `New note`, not the whole label.
- The chip advertises `⌘⌥N`. The harness presses `Alt+Meta+n`. `Ctrl+Alt+N` is the same shortcut on Windows and Linux (`Alt+Ctrl+n`). Plain `⌘N` does not create a note.
- Press the chord only when focus is outside a text field. A focused search box does not create a note.
- **+ New note** is absent while a search query or a tag filter is active. Create from the `Home` heading.
- The stored name gains a `.md` suffix. The list and the title show the name without that suffix.
- A navigation to `/note/` is not persistence. Reopen the note from the drive list.
- The title is the local time to the second. A second create in that same second uses the same title.
- A new note body is `# `. It does not contain the timestamp title. `browser insert` types at the caret, so `# Hello` shows the caret started after the space.
