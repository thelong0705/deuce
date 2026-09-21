# deuce web

Signup and sign-in interface. Vite + React + TypeScript.

```bash
npm install
npm run dev
```

Opens on http://localhost:5173 and proxies `/users` and `/sessions` to
`http://localhost:8080`. Override the backend with
`API_URL=http://host:port npm run dev`.

```bash
npm run typecheck
npm run build
```

## The contract this expects

```
POST /users
{ "email": "...", "password": "...", "phone_number": "...", "role": "player" | "owner" }

201 -> { "id", "email", "display_name", "role", "created_at" }
400 -> { "code": "password_too_short", "error": "..." }
409 -> { "code": "email_taken", "error": "..." }

POST /sessions
{ "email": "...", "password": "..." }

201 -> { "user_id", "expires_at" } + Set-Cookie: deuce_session=...
401 -> { "code": "invalid_credentials", "error": "..." }

DELETE /sessions

204 -> Set-Cookie: deuce_session=; Max-Age=0
```

Client-side validation mirrors `internal/domain/entity`: a valid email, a
password of 8 to 72 bytes, and a non-empty phone number. Password length is
measured in bytes, not characters, because Go's `len()` does — so an accented
password of 70 characters is over the limit in both places. The sign-in form
only checks that both fields are filled: an existing password is the server's
to judge, and "too short" on a sign-in form is a hint about a password nobody
has proven they own.

## How the session is held

The token lives in the `deuce_session` cookie, which is `httpOnly`, so this
code never sees it. `src/session.ts` keeps a copy of the user id and expiry in
`localStorage` purely so a reload can render the signed-in view without a
flash — it is a hint, not a credential, and a 401 drops it.

Registering does not start a session. After signup the form hands the email to
the sign-in tab and the user logs in.

### Safari on plain HTTP

The cookie is set with `Secure`. Chrome and Firefox treat `http://localhost` as
a secure context and store it anyway; **Safari does not**, so sign-in appears
to succeed and the next request arrives without a cookie. Use Chrome or Firefox
for local development, or serve the API over HTTPS.

## Not wired up yet

There is no `GET /users/me`, so a reload trusts the stored hint until a request
comes back 401. Once the auth middleware lands, that check should happen on
mount instead.
