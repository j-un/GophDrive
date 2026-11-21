# Demo login

Demo login gives the visitor a private, expiring drive without Google. The home page sends them to the drive, which already contains two notes.

## Sub-features

- `demo-home` shows the signed-out home page once Wasm is ready.
- `demo-enter` follows **Try Demo Mode** and lands on the drive.
- `demo-seed` shows the two seeded notes.
- `demo-who` shows a `demo-user-` id on Settings.

## How to get to it (user POV)

- Open `http://localhost:5173/`.
- Choose **Try Demo Mode**.
- Choose **Settings** in the sidebar to see the user id.

## Driving it with gophdrive-verify

Preconditions:

- `doctor` prints `worth driving: yes`.
- `browser start` has launched the verification Chrome.
- The page is signed out. A leftover `session_token` from an earlier navigation in this same Chrome must be absent; `cleanup` then `browser start` is a new profile.

- **Open home.** Go to the app. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser goto http://localhost:5173/`. The heading is `GophDrive` and the link `Try Demo Mode` is visible. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Try Demo Mode"`.
- **Wait for Wasm.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Core Module Active" --timeout 45000`. The status reads `Core Module Active`.
- **Enter demo.** Choose **Try Demo Mode**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role link --name "Try Demo Mode"`. The address becomes `/drive/`. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /drive/ --timeout 30000`.
- **Seeded notes.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "Welcome!"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-text --text "ようこそ!"`. The heading is `Home`. Both notes are listed.
- **Session cookie.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser cookies`. Output includes `session_token` on `localhost`. Do not record the cookie value.
- **User id.** Choose **Settings**. Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser click --role button --name "Settings"` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser wait-url --includes /settings/`. The page heading is `Settings` and the user id starts with `demo-user-`.
- **Proof.** Run `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser snapshot --path .cursor/skills/verify-gophdrive/evidence/demo-login/settings.txt` and `mise x node -- node .cursor/skills/verify-gophdrive/scripts/gophdrive-verify.mjs browser screenshot --path .cursor/skills/verify-gophdrive/evidence/demo-login/settings.png`. The artifacts show `Settings` and a `demo-user-` id.

## Gotchas

- **Login with Google** is the wrong control. It leaves localhost for Google.
- Opening `127.0.0.1` drops the session. The cookie is set for `localhost` only, and it is `Secure` and `HttpOnly`.
- Click **Try Demo Mode** only after `Core Module Active`. The link is present while Wasm is still loading; preview later in the session needs the module.
- The drive redirects to `/` when the cookie is missing. A home page after the click means the cookie was not stored.
- Settings has two **Settings** controls when the sidebar is expanded (the rail icon and the panel button). Either opens `/settings/`. The harness clicks the visible one whose accessible name matches.
