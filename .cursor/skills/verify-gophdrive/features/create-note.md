# Create a note

Create note asks for a name, opens the new note in the editor, and shows that note again from the drive list. Escape drops an unsaved name.

## Sub-features

- `create-button` opens the name field from **+ New note**.
- `create-shortcut` opens the same field from Ctrl+N when focus is outside a text field.
- `create-cancel` discards the draft with Escape.
- `create-save` creates the note and opens it.
- `create-persist` shows the new note in the drive list and opens it again.

## How to get to it (user POV)

- Choose **+ New note** on the drive (the chip reads `⌘N`).
- Press `⌘N` or `Ctrl+N` while focus is outside a text field.
- The name field is the textbox whose placeholder is `Note name...`. Enter saves. Escape cancels.

## Driving it with gophdrive-verify

Preconditions:

- Baseline demo login has landed on `/drive/` with heading `Home`.
- No note is named `Release checklist` or `Discard me`. A fresh demo user satisfies this.

- **Open from the button.** Choose **+ New note**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "New note"`. A textbox with placeholder `Note name...` is focused.
- **Cancel the draft.** Type a name and press Escape. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Note name..." --value "Discard me"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Escape`. The name field closes. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "Discard me" --timeout 5000`. The list has no `Discard me` link.
- **Open from the keyboard.** Press Ctrl+N. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Ctrl+n`. The `Note name...` field is back. Focus must not be in a text field when the key is pressed; Escape leaves focus outside one.
- **Name the note.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Note name..." --value "Release checklist"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`.
- **Editor opens.** The address contains `/note/` and the title reads `Release checklist`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/ --timeout 15000` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Release checklist"`. The editor body contains `# Release checklist`.
- **Capture the editor.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/create-note/editor.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/create-note/editor.png`.
- **Return to the list.** Choose **Back to Drive**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Back to Drive"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/`. The list includes `Release checklist` and still includes `Welcome!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Release checklist"`.
- **Reopen.** Choose the `Release checklist` link. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Release checklist"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/`. The title is `Release checklist` again.
- **Proof.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/create-note/reopened.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/create-note/reopened.png`. Both artifacts show `Release checklist` on `/note/`.

## Gotchas

- The accessible name of **+ New note** includes the `⌘N` chip. Match `New note`, not the whole label.
- `⌘N` is what the chip advertises. Chrome handles `⌘N` as a new window, so the harness presses `Ctrl+n`. The page treats both as the shortcut. Do not claim the Meta chord from a Ctrl run.
- Ctrl+N while the name field or the search box is focused types into that field. Press it only when focus is outside a text field.
- Escape discards the draft even when the field contains text. Blur with an empty field also closes it.
- The stored name gains a `.md` suffix. The list and the title show the name without that suffix.
- **+ New note** is absent while a search query or a tag filter is active. Create from the `Home` heading, not from search results.
- A navigation to `/note/` is not persistence. Reopen the note from the drive list.
