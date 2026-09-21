import { useState } from 'react'

import { ApiError, logout } from './api'
import type { Session } from './api'

type Props = {
  session: Session
  onSignedOut: () => void
}

export function SignedIn({ session, onSignedOut }: Props) {
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

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
    <div className="card">
      <h1>Signed in</h1>
      <p className="lede">Your session is active.</p>

      <dl className="summary">
        <dt>User</dt>
        <dd className="mono">{session.user_id}</dd>
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
  )
}

function formatExpiry(value: string): string {
  const at = new Date(value)
  if (Number.isNaN(at.getTime())) {
    return value
  }

  return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
