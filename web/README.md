# deuce web

Signup interface. Vite + React + TypeScript.

```bash
npm install
npm run dev
```

Opens on http://localhost:5173 and proxies `/users` to `http://localhost:8080`.
Override the backend with `API_URL=http://host:port npm run dev`.

```bash
npm run typecheck
npm run build
```

## Status

**There is no signup endpoint yet.** `POST /users` does not exist on the Go
side, so submitting the form returns "The signup endpoint does not exist yet."
The form, its validation and its error handling all work; only the request
fails.

## The contract this expects

```
POST /users
{ "email": "...", "password": "...", "phone_number": "...", "role": "player" | "owner" }

201 -> { "id", "email", "display_name", "role", "created_at" }
400 -> { "error": "password must be at least 8 characters" }
409 -> { "error": "email already registered" }
```

Client-side validation mirrors `internal/domain/entity`: a valid email, a
password of 8 to 72 bytes, and a non-empty phone number. Password length is
measured in bytes, not characters, because Go's `len()` does — so an accented
password of 70 characters is over the limit in both places.
