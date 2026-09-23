# deuce web

Signup, sign-in and venue registration. Vite + React + TypeScript.

```bash
npm install
npm run dev
```

Opens on http://localhost:5173 and proxies the API paths to
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

POST /venues            (session cookie required)
{ "name": "...", "city": "...", "address": "..." }

201 -> { "id", "owner_id", "name", "city", "address", "timezone",
         "is_active", "created_at" }
400 -> { "code": "venue_city_unsupported", "error": "..." }
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

## Cities

Both the venue form and the player's search offer a select filled from
`GET /cities` rather than a free-text field. The set is master data — a venue
can only be in one of them, and a search only finds venues in one of them, so
a typed city could only ever be wrong.

A city name can contain a space, so it is URL-encoded on the way into a search:
`?city=Ha%20Noi`.

## Courts

Each venue can expand an inline form to add a court: a name, opening and
closing hours, and a price per hour. The hour selects are bounded at the
source — 00:00 to 23:00 for opening, 01:00 to 24:00 for closing — so the only
ordering rule left to state is open before close.

Those hours are read in the venue's timezone, which the city decides — so
registering a venue does not ask for one, and the response carries the zone
back without the form ever having sent it.

`GET /venues/{venueID}/courts` reads them back, so the list under a venue
survives a reload.

## Booking

A player sees the booking screens where an owner sees venues: search a city,
open a venue, pick a court, pick a two-hour window, book.

Booking a slot does not book it outright: it takes the slot and opens a
payment.

```
POST /courts/{courtID}/bookings
{ "starts_at": "2026-09-22T06:00:00+07:00" }

201 -> { "id", "court_id", "player_id", "starts_at", "ends_at",
         "status", "amount", "created_at", "client_secret" }
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

### The rest of the flow

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

`starts_at` goes out exactly as the availability response gave it. Which real
instant "06:00" means depends on the venue's timezone, so having the browser
construct one from its own clock would get it wrong for anyone travelling or
for a venue in another zone. The date picker sends a plain `YYYY-MM-DD`;
everything finer round-trips.

The date input is bounded to today through fourteen days out, matching the
booking horizon. The server still has to enforce it, along with the minimum
lead time, which the UI does not attempt to police.

### Safari on plain HTTP

The cookie is set with `Secure`. Chrome and Firefox treat `http://localhost` as
a secure context and store it anyway; **Safari does not**, so sign-in appears
to succeed and the next request arrives without a cookie. Use Chrome or Firefox
for local development, or serve the API over HTTPS.

## Paying

`POST /courts/{courtID}/bookings` holds the slot and returns a
`client_secret`. The browser pays with Stripe's Payment Element, and **Stripe
tells the server the result, not this code** — the webhook is what moves the
booking to `confirmed`. Nothing here can confirm a booking, which is the
point: a client that could would be a client that could book for free.

So the UI only ever reports what it saw. `confirmPayment` succeeding shows
"paid and booked"; anything else says the payment is still going through and
leaves the list to catch up.

An unpaid hold lapses after fifteen minutes. Until then it appears under
**Your bookings** marked *Awaiting payment*, because a held slot that looked
like a booking would be a lie about something someone owes money on.

### The publishable key

`VITE_STRIPE_PUBLISHABLE_KEY` in `.env.development` and `.env.production`,
both committed. It is safe in the bundle by design — it can start a payment
and nothing else — so a file everyone can read is where it belongs, and a
build that has to be handed the value is a build that can silently ship
without it.

Without it the booking still holds the slot, and the panel says payments are
not configured rather than rendering an empty box.

## Not wired up yet

Nothing polls after a payment. `confirmPayment` returning `succeeded` is taken
at its word for the message on screen, and **Your bookings** shows whatever the
next load finds — so a booking confirmed by the webhook a second later reads as
*Awaiting payment* until the section is opened again. Refetching the list a few
times after a payment, or having the server hold the response until the webhook
lands, would close that.

Nothing cancels a hold either. Walking away leaves the slot held for fifteen
minutes; a "not now" only closes the panel.
