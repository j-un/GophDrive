# Search notes

Search finds notes by title or body from the sidebar field, opens a hit, and shows an empty state when nothing matches. Leaving search returns to the folder list.

## Sub-features

- `search-field` runs a query from the sidebar textbox.
- `search-shortcut` focuses that textbox from Ctrl+K.
- `search-title` matches `Welcome!` and does not match `ようこそ!` for the query `Welcome`.
- `search-body` matches `Welcome!` for a word that is only in its body.
- `search-empty` shows `No notes found.` for a query with no hit.
- `search-leave` returns to the `Home` list. There is no clear button.

## How to get to it (user POV)

- Type in the sidebar textbox whose placeholder is `Search...`, then press Enter.
- Press `⌘K` or `Ctrl+K` to focus that textbox, then type and press Enter.
- Choose a result's name to open the note.
- Choose the **GophDrive** logo or **Notes** to leave search.

## Driving it with gophdrive-verify

Preconditions:

- Baseline demo login has landed on `/drive/` with `Welcome!` and `ようこそ!` visible.
- The sidebar is expanded (a fresh profile is expanded). The search textbox is visible without opening a menu.

- **Focus from the keyboard.** Press Ctrl+K. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Ctrl+k`. The `Search...` textbox is focused.
- **Title match.** Type `Welcome` and press Enter. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Search..." --value "Welcome"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`. The address contains `q=Welcome` and the heading is `Search results for "Welcome"`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Search results for"`. The list contains `Welcome!` and does not contain `ようこそ!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-absent --text "ようこそ!" --timeout 5000`.
- **Open the hit.** Choose `Welcome!`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Welcome!"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /note/`. The note title is `Welcome!`.
- **Body match.** Return to the drive, focus search, and query a body word. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Back to Drive"`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Ctrl+k`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Search..." --value "blockquote"`, and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`. The heading is `Search results for "blockquote"` and `Welcome!` is listed. `blockquote` is not part of the note's name.
- **Empty query.** Replace the query with `volcano` and press Enter. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Search..." --value "volcano"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`. The page shows `No notes found.` Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "No notes found."`.
- **Leave search.** Choose the **GophDrive** logo. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "GophDrive"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Home"`. The heading is `Home` and both seeded notes are listed. The address has no `q=`.
- **Proof.** Re-run the title query and capture it before opening a hit. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser fill --placeholder "Search..." --value "Welcome"`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser press --key Enter`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Search results for"`, `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/search/results.txt`, and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/search/results.png`. The artifacts show the query and `Welcome!`.

## Gotchas

- The field does not search as you type. Nothing changes until Enter. The address becomes `/drive/?q=`.
- `⌘K` focuses search. The harness sends `Ctrl+k` for the same listener. Do not claim the Meta chord from a Ctrl run.
- Ctrl+K while a text field is focused still runs the shortcut; the page listener does not ignore inputs for this chord. Ctrl+N does ignore inputs.
- **+ New note** disappears while a query is active. That is the search page, not a missing button.
- Seeded notes have no tags. A `tag:` query matches nothing until a note body contains that tag. Do not treat an empty tag query as a broken search.
- `Welcome` matches the English note's name. `blockquote` matches its body (`This is a blockquote`) and is not in the title. Assert the heading `Search results for "blockquote"` so a name-only match is not mistaken for a body match.
- There is no **Clear search** control. Leave via the **GophDrive** logo (`aria-label` `GophDrive`) or the **Notes** rail button.
