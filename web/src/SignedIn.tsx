import { useEffect, useState } from 'react'

import { Venues } from './Venues'
import { ApiError, logout, me } from './api'
import type { Session, User } from './api'

type Props = {
  session: Session
  onSignedOut: () => void
}

export function SignedIn({ session, onSignedOut }: Props) {
  // null until /me answers. The stored session hint says nothing about the
  // role, so who this is has to come from the server.
  const [user, setUser] = useState<User | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    let cancelled = false

    void (async () => {
      try {
        const current = await me()
        if (!cancelled) {
          setUser(current)
        }
      } catch (err) {
        // This is also the first real check that the session is still live.
        if (err instanceof ApiError && err.status === 401) {
          onSignedOut()
        } else if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Could not load your account.')
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [onSignedOut])

  async function handleLogout() {
    setSubmitting(true)
    setError(null)
    try {
      await logout()
      onSignedOut()
    } catch (err) {
      // A 401 means the session was already gone, which is the outcome we
      // wanted. Anything else left the row in place, so stay signed in and say
      // so rather than pretending.
      if (err instanceof ApiError && err.status === 401) {
        onSignedOut()
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Something went wrong.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <div className="card">
        <h1>Signed in</h1>
        <p className="lede">Your session is active.</p>

        <dl className="summary">
          <dt>User</dt>
          <dd>{user ? user.email : <span className="mono">{session.user_id}</span>}</dd>
          <dt>Account</dt>
          <dd>{user ? accountLabel(user) : '…'}</dd>
          <dt>Expires</dt>
          <dd>{formatExpiry(session.expires_at)}</dd>
        </dl>

        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}

        <button type="button" className="secondary" onClick={handleLogout} disabled={submitting}>
          {submitting ? 'Signing out…' : 'Sign out'}
        </button>
      </div>

      {/* Venues are an owner's feature, so a player is never shown the section
          at all — not an empty list and a form that would only 403. */}
      {user?.role === 'owner' && <Venues onUnauthorized={onSignedOut} />}
    </>
  )
}

function accountLabel(user: User): string {
  return user.role === 'owner' ? 'Court owner' : 'Player'
}

function formatExpiry(value: string): string {
  const at = new Date(value)
  if (Number.isNaN(at.getTime())) {
    return value
  }

  return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
