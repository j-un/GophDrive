# Local Google OAuth setup

The default `.env` ships dummy Google credentials, so `./scripts/dev.sh`
runs end-to-end only via the **Demo login** path. That covers most code
but skips four real-auth surfaces:

- ID token signature/audience validation (`auth.GoogleVerifier`)
- Email allow-list (`isEmailAllowed`)
- The cookie-based session round-trip
- The OAuth state / redirect handshake

To exercise these locally, populate `.env` with a real Google OAuth client.
**No code changes are needed** — `app.NewApp` already auto-routes the dev
redirect to `http://localhost:8080/auth/callback`.

## One-time setup in Google Cloud

1. **Create or pick a project** in the
   [Google Cloud Console](https://console.cloud.google.com/).
2. **OAuth consent screen** → User type: *External*, Publishing status:
   *Testing*. Add your own Google account under **Test users**. Scopes
   needed: `openid`, `.../auth/userinfo.email`,
   `.../auth/userinfo.profile`. (No Drive scope — it was removed when
   storage moved to DynamoDB.)
3. **Credentials** → *Create credentials* → *OAuth client ID* →
   Application type: *Web application*.
   - **Authorized redirect URIs**: add exactly
     `http://localhost:8080/auth/callback` (no trailing slash, http not https).
   - **Authorized JavaScript origins**: `http://localhost:3000`.
4. Copy the **Client ID** and **Client secret** that the console shows
   you after creation.

## Wiring `.env`

Edit the repo's `.env` (gitignored — it's per-developer):

```env
GOOGLE_CLIENT_ID=<paste from console>
GOOGLE_CLIENT_SECRET=<paste from console>
```

Optionally lock the allow-list to your address so you can also exercise
the deny path:

```env
ALLOWED_EMAILS=you@example.com
```

Restart the backend so overmind reloads `.env`:

```bash
overmind connect backend       # Ctrl-C inside the pane
overmind restart backend       # from another shell
```

## Verifying

1. Open `http://localhost:3000`.
2. Click **Login with Google** (not the demo button).
3. You should be redirected to Google → consent screen → back to
   `localhost:8080/auth/callback` → finally to
   `localhost:3000/?success=true`. A `session_token` cookie should be
   present (DevTools → Application → Cookies).
4. The note list should show the auto-minted `GophDrive` root folder
   for your real Google subject ID.

To exercise the deny path: temporarily set
`ALLOWED_EMAILS=someone-else@example.com` and retry — the callback
should respond `403 Access denied` instead of redirecting.

## Troubleshooting

- **`redirect_uri_mismatch`** — the redirect URI in the Google console
  must match exactly: scheme, host, port, path. `http`, not `https`.
- **`invalid_client`** — `GOOGLE_CLIENT_SECRET` mismatch, or you copied
  the secret from a different OAuth client.
- **Stuck on consent screen with "App isn't verified"** — expected in
  *Testing* status; click *Advanced → Continue*. Verification is only
  needed if you publish to real users.
- **Loops back to `/?success=true` but `getUser` returns 401** — check
  that `JWT_SECRET` in `.env` is non-empty; the callback signs the
  session JWT with it.
- **Backend can't reach Google JWKS** — `auth.GoogleVerifier.Verify`
  calls `googleapis.com/oauth2/v3/certs`. If the dev host is offline
  or behind a proxy, use Demo login instead.

## Reverting

Delete (or replace with `dummy`) the `GOOGLE_CLIENT_ID` /
`GOOGLE_CLIENT_SECRET` lines, restart the backend, and Demo login is
the only working path again. There's no other state to clean up.
