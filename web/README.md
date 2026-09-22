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
{ "name": "...", "city": "...", "address": "...", "timezone": "Asia/Ho_Chi_Minh" }

201 -> { "id", "owner_id", "name", "city", "address", "timezone",
         "is_active", "created_at" }
400 -> { "code": "venue_timezone_invalid", "error": "..." }
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

Those hours are read in the venue's timezone, which is why registering a venue
asks for one. The select is filled from `Intl.supportedValuesOf('timeZone')`
and defaults to the browser's own zone, since an owner is nearly always
registering a venue where they are sitting. Where the browser will not
enumerate zones, a short list stands in and the browser's own zone is added to
it, so the default is never missing from its own list.

**There is no endpoint to read courts back** for an owner. Only `POST` exists,
so the list under each venue holds what was added in this session and starts
empty on every reload. The `GET /venues/{venueID}/courts` proposed below would
fix that too, and the list would then work like the venue list does.

## Booking

A player sees the booking screens where an owner sees venues: search a city,
open a venue, pick a court, pick a two-hour window, book.

**Only the booking itself is implemented.** Everything a player needs to reach
one — browsing, listing courts, availability, and their own bookings — is
still a proposal, returns 404 today, and the screens say so. This is the same
way the signup form started.

```
POST /courts/{courtID}/bookings        (implemented)
{ "starts_at": "2026-09-22T06:00:00+07:00" }

201 -> { "id", "court_id", "player_id", "starts_at", "ends_at", "created_at" }
400 -> { "code": "slot_not_on_the_hour" | "slot_not_on_the_grid"
                | "slot_in_the_past" | "slot_too_far_ahead"
                | "slot_outside_opening_hours"
                | "invalid_starts_at" }
403 -> { "code": "court_inactive" | "not_a_player" | "player_inactive" }
404 -> { "code": "court_not_found" }
409 -> { "code": "slot_taken" }
```

The court is named by the path, so the body is only the hour, and the response
does not repeat the court beyond its id. `slot_taken` is the one the
`bookings (court_id, starts_at) WHERE cancelled_at IS NULL` index enforces;
the UI reloads the times and says someone just took it.

### Still proposed

```
GET /venues/search?city=...            (session cookie required)

200 -> { "venues": [ { venue fields } ] }
```

Distinct from `GET /venues`, which is owner-scoped and only ever returns the
caller's own. Browsing has to see everybody's.

```
GET /venues/{venueID}/courts           (session cookie required)

200 -> { "courts": [ { court fields } ] }

GET /courts/{courtID}/availability?date=YYYY-MM-DD

200 -> { "slots": [ { "starts_at": "2026-09-22T06:00:00+07:00",
                      "ends_at":   "2026-09-22T08:00:00+07:00",
                      "available": true } ] }
```

One entry per bookable two-hour window. Windows tile the day from the court's
`open_hour`, so 06:00-10:00 offers 06:00-08:00 and 08:00-10:00 and nothing in
between; `available` is false where an uncancelled booking already holds one.
Each entry carries its own `ends_at`, so the client never encodes how long a
slot runs.

```
GET /bookings                          (session cookie required)

200 -> { "bookings": [ { booking fields,
                         "court": { "name", "price_per_hour" },
                         "venue": { "id", "name", "city" } } ] }
```

The booking fields are the ones `POST` returns. The court and venue names are
added because a list of court ids tells a player nothing, and resolving them
in the client would be one request per row.

### Why the client never builds a timestamp

`starts_at` goes out exactly as the availability response gave it. A venue has
no timezone column, so which real instant "06:00" means is a question only the
server can answer — having the browser construct one from its own clock would
get it wrong for anyone travelling or for a venue in another zone. The date
picker sends a plain `YYYY-MM-DD`; everything finer round-trips.

The date input is bounded to today through fourteen days out, matching the
booking horizon. The server still has to enforce it, along with the minimum
lead time, which the UI does not attempt to police.

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
