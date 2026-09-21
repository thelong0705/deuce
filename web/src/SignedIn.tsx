import { useEffect, useState } from 'react'

import { Account } from './Account'
import { Browse } from './Browse'
import { MenuBar } from './MenuBar'
import type { MenuItem } from './MenuBar'
import { MyBookings } from './MyBookings'
import { Venues } from './Venues'
import { ApiError, logout, me } from './api'
import type { Session, User } from './api'

type Section = 'book' | 'bookings' | 'venues' | 'account'

const playerMenu: MenuItem<Section>[] = [
  { key: 'book', label: 'Book a court' },
  { key: 'bookings', label: 'My bookings' },
  { key: 'account', label: 'Account' },
]

const ownerMenu: MenuItem<Section>[] = [
  { key: 'venues', label: 'My venues' },
  { key: 'account', label: 'Account' },
]

type Props = {
  session: Session
  onSignedOut: () => void
}

export function SignedIn({ session, onSignedOut }: Props) {
  // null until /me answers. The stored session hint says nothing about the
  // role, so who this is has to come from the server.
  const [user, setUser] = useState<User | null>(null)
  const [section, setSection] = useState<Section>('account')
  const [error, setError] = useState<string | null>(null)
  const [signingOut, setSigningOut] = useState(false)
  // Bumped after a booking so My bookings reloads rather than showing what it
  // happened to fetch earlier.
  const [version, setVersion] = useState(0)

  useEffect(() => {
    let cancelled = false

    void (async () => {
      try {
        const current = await me()
        if (cancelled) {
          return
        }
        setUser(current)
        // Land on the section that account is actually for.
        setSection(current.role === 'owner' ? 'venues' : 'book')
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

  async function handleSignOut() {
    setSigningOut(true)
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
      setSigningOut(false)
    }
  }

  // Venues are an owner's feature and booking is a player's, so neither is
  // offered to the other — not an empty list, and not a form that could only
  // ever come back 403.
  const items = user === null ? [] : user.role === 'owner' ? ownerMenu : playerMenu

  return (
    <>
      <MenuBar
        user={user}
        items={items}
        active={section}
        onSelect={setSection}
        onSignOut={handleSignOut}
        signingOut={signingOut}
      />

      <main className="page page-below-menu">
        {error && (
          <p className="card form-error" role="alert">
            {error}
          </p>
        )}

        {section === 'account' && <Account session={session} user={user} />}
        {section === 'venues' && user?.role === 'owner' && <Venues onUnauthorized={onSignedOut} />}
        {section === 'book' && user?.role === 'player' && (
          <Browse onBooked={() => setVersion((v) => v + 1)} onUnauthorized={onSignedOut} />
        )}
        {section === 'bookings' && user?.role === 'player' && (
          <MyBookings version={version} onUnauthorized={onSignedOut} />
        )}
      </main>
    </>
  )
}
