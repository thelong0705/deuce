import type { Session } from './api'

const storageKey = 'deuce.session'

// The real credential is the httpOnly session cookie, which this code cannot
// read. What is stored here is only a hint so a page reload can render the
// signed-in view without flicker; the server is still the one that decides.
// Any 401 means the hint is stale and should be dropped.

export function loadSession(): Session | null {
  let raw: string | null
  try {
    raw = window.localStorage.getItem(storageKey)
  } catch {
    // Storage can be blocked entirely; a signed-in user just starts signed out.
    return null
  }

  if (raw === null) {
    return null
  }

  const session = parse(raw)
  if (session === null || Date.parse(session.expires_at) <= Date.now()) {
    clearSession()
    return null
  }

  return session
}

export function saveSession(session: Session): void {
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(session))
  } catch {
    // Not being able to remember the session is survivable.
  }
}

export function clearSession(): void {
  try {
    window.localStorage.removeItem(storageKey)
  } catch {
    // Same.
  }
}

function parse(raw: string): Session | null {
  try {
    const value: unknown = JSON.parse(raw)
    if (typeof value === 'object' && value !== null) {
      const { user_id, expires_at } = value as Record<string, unknown>
      if (typeof user_id === 'string' && typeof expires_at === 'string') {
        return { user_id, expires_at }
      }
    }
  } catch {
    // Corrupt or hand-edited; treat it as absent.
  }

  return null
}
