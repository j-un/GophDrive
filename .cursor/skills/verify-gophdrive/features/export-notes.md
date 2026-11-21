# Export notes

Export downloads a ZIP of every note in the signed-in drive. Folder names are paths inside the archive. The demo drive's seeded notes are in that ZIP.

## Sub-features

- `export-open` opens Settings from the sidebar.
- `export-zip` downloads one `.zip` after **Export all notes (.zip)**.
- `export-contains` the archive includes the seeded notes.

## How to get to it (user POV)

- Choose **Settings** in the sidebar rail, or the **Settings** button at the bottom of the sidebar panel.
- Under **Export**, choose **Export all notes (.zip)**. The browser downloads a ZIP.
- Choose **Back** to leave Settings.

## Driving it with gophdrive-verify

Preconditions:

- Baseline demo login has landed on `/drive/` with `Welcome!` visible.
- `browser start` has already allowed downloads into the skill's scratch download directory.

- **Open Settings.** Choose **Settings**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "Settings"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /settings/`. The heading is `Settings` and the button reads `Export all notes (.zip)`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Export all notes (.zip)"`.
- **Download.** Choose **Export all notes (.zip)**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "Export all notes (.zip)"`. The button label becomes `Exporting...` while the request is in flight, then returns to `Export all notes (.zip)`. A `.zip` file lands in the scratch download directory.
- **Keep the file.** Copy it before cleanup. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser downloads --copy-to .cursor/skills/verify-gophdrive/evidence/export`. The command prints the copied path and exits `0` only when a completed file exists.
- **Look inside.** Unzip the copied archive to a scratch directory and list the paths. `Welcome!.md` and `ようこそ!.md` are both present. A missing seeded note means the export did not include the drive you are looking at.
- **Proof.** Keep the ZIP at `evidence/export/`. Also run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/export/settings.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/export/settings.png` after the button has returned to `Export all notes (.zip)`. The screenshot alone is not the proof.

## Gotchas

- The scratch download directory is under `.cursor/skills/verify-gophdrive/run/` and cleanup deletes it. Copy to `evidence/export/` first.
- Demo users cannot create API keys. The API key section on Settings is not part of this recipe.
- **Logout** asks for confirmation. The confirm button is also named `Logout`. Do not click it during an export proof; it drops the session.
- **Back** is `aria-label` `Back` and uses browser history. It is not required to prove the ZIP.
