# deuce web

Signup, sign-in and venue registration. Vite + React + TypeScript.

```bash
npm install
npm run dev
```

Opens on http://localhost:5173 and proxies `/users`, `/sessions` and `/venues`
to `http://localhost:8080`. Override the backend with
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

POST /venues            (session cookie required)
{ "name": "...", "city": "...", "address": "..." }

201 -> { "id", "owner_id", "name", "city", "address", "is_active", "created_at" }
401 -> { "code": "session_invalid", "error": "..." }
403 -> { "code": "not_an_owner", "error": "..." }

GET /venues             (session cookie required)

200 -> { "venues": [ ... ] }

POST /venues/{venueID}/courts          (session cookie required)
{ "name": "...", "open_hour": 6, "close_hour": 22, "price_per_hour": 150 }

201 -> { "id", "venue_id", "name", "open_hour", "close_hour",
         "price_per_hour", "is_active", "created_at" }
403 -> { "code": "not_venue_owner", "error": "..." }
409 -> { "code": "court_name_taken", "error": "..." }
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

## Venues

The signed-in view lists the venues the user owns and offers a form to add
one. Nothing in the session says whether the account is a player or an owner,
so the form is offered to everyone; a player's `POST /venues` comes back 403
`not_an_owner` and the form says so in plain words. Once the session carries a
role, the form can be hidden instead.

Neither call names an owner. Both venue routes sit behind the auth middleware,
so the server takes the owner from the session cookie — the client cannot ask
for someone else's venues by editing a query string.

Which of the two an account sees is decided by `GET /me`: the venues section
is rendered only for an owner. A player is not shown a form whose only
possible answer is 403. `VenueForm` still handles `not_an_owner` anyway — a
role can change while a session is open, and this is presentation, not
enforcement.

## Courts

Each venue can expand an inline form to add a court: a name, opening and
closing hours, and a price per hour. The hour selects are bounded at the
source — 00:00 to 23:00 for opening, 01:00 to 24:00 for closing — so the only
ordering rule left to state is open before close.

**There is no endpoint to read courts back.** Only `POST` exists, so the list
under each venue holds what was added in this session and starts empty on
every reload. A `GET /venues/{venueID}/courts` would fix that, and the list
would then work like the venue list does.

### Safari on plain HTTP

The cookie is set with `Secure`. Chrome and Firefox treat `http://localhost` as
a secure context and store it anyway; **Safari does not**, so sign-in appears
to succeed and the next request arrives without a cookie. Use Chrome or Firefox
for local development, or serve the API over HTTPS.

## Not wired up yet

`GET /me` exists but nothing calls it. A reload still trusts the stored hint
until some request comes back 401 — on the signed-in view that happens
immediately, because loading the venue list is the first thing it does. Calling
`/me` on mount would confirm the session directly and, since it returns the
role, let the venue form be hidden from players instead of letting them find
out by submitting it.
