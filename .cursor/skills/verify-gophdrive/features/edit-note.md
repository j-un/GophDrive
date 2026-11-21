# Edit a note

Edit a note types into the open editor, waits until the page reports the save, and reads the same text back after leaving. Preview shows the rendered note instead of the Markdown source.

## Sub-features

- `edit-open` opens a note from the drive list.
- `edit-type` inserts text at the caret and shows `unsaved`, then a `saved` time.
- `edit-persist` shows that text after the note is closed and opened again.
- `edit-preview` swaps the source for rendered Markdown. The toggle's name becomes `Edit`.

## How to get to it (user POV)

- On the drive, choose the note's name (`Welcome!` is seeded).
- Click in the editor and type. The status next to the title changes from `unsaved` to `saved` plus a 24-hour time.
- Choose the eye button, accessible name **Preview**. Choose it again (now named **Edit**) to return to the source.
- Choose **Back to Drive**, then choose the note again.

## Driving it with gophdrive-verify

Preconditions:

- Baseline demo login has landed on `/drive/`.
- `Welcome!` is in the list. Do not edit a note whose body must stay pristine for a later assertion in the same session; this recipe changes `Welcome!`.

- **Open the note.** Choose `Welcome!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Welcome!"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/`. The title is `Welcome!` and the source starts with `# Welcome to GophDrive!`.
- **Type.** Put the marker on its own line. A just-opened note has the caret at the start of `# Welcome to GophDrive!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser insert --text "VERIFIED"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`. The first line is `VERIFIED`. The next line is still `# Welcome to GophDrive!`. The status reads `unsaved`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "unsaved"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "VERIFIED #"`.
- **Autosave.** Wait out the two-second save. The status becomes `saved` plus a time such as `saved 18:24`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "saved " --timeout 10000`. `saved ` with the trailing space does not match `unsaved`.
- **Leave and return.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Back to Drive"`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Welcome!"`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "VERIFIED"`, and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "VERIFIED #"`. The reopened source starts with `VERIFIED` on its own line. The next line is `# Welcome to GophDrive!`.
- **Preview.** Choose **Preview**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "Preview"`. The button's name becomes `Edit`, and the page text has no `#`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-name --role button --name "Edit"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "#"`.
- **Proof.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/edit-note/preview.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/edit-note/preview.png`. The snapshot contains the line `h1: Welcome to GophDrive!` and the line `button "Edit"`. The screenshot shows that heading with no leading `#`, and `VERIFIED` above it.

## Gotchas

- Autosave waits 2 seconds after the last change. Wait for `saved `, not a fixed sleep alone, and not the substring `saved` (that also matches `unsaved`).
- `browser insert` focuses `.cm-content` and inserts at the caret. It does not move the caret to the end of the note. A just-opened note takes the insert on the first line.
- Preview hides the editor (`display: none`), so the page text is the rendered note. The toggle's accessible name is `Preview` when the source is showing and `Edit` when the preview is showing. Assert the toggle name with `wait-name`, not `wait-text`.
- If the home page never reached `Core Module Active`, preview renders nothing. Do not treat a blank preview as an empty note.
- The status can also read `error`. That is a failed save, not success.
