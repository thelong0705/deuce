import { useCallback, useEffect, useState } from 'react'

import { PendingBooking } from './PendingBooking'
import { ApiError, listBookings } from './api'
import type { BookingListItem } from './api'

type Props = {
  // version changes when a booking is made, which reloads the list.
  version: number
  onUnauthorized: () => void
}

// Long enough for a webhook that is slow rather than lost. Past this the
// booking is still paid for and still confirms; only this page stops watching.
const confirmAttempts = 10
const confirmInterval = 1000

export function MyBookings({ version, onUnauthorized }: Props) {
  const [bookings, setBookings] = useState<BookingListItem[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (): Promise<BookingListItem[] | null> => {
    setError(null)
    try {
      const latest = await listBookings()
      setBookings(latest)
      return latest
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return null
      }
      // Not the same as having nothing booked, so do not say that.
      setError(err instanceof ApiError ? err.message : 'Could not load your bookings.')
      return null
    }
  }, [onUnauthorized])

  // A card that has gone through is not a booking yet: Stripe's webhook is what
  // confirms it, and that lands a moment after the browser hears back. Loading
  // once would almost always ask too early and leave a paid slot looking held,
  // so the list is reloaded until the hold is gone.
  const awaitConfirmation = useCallback(
    async (id: string) => {
      for (let attempt = 0; attempt < confirmAttempts; attempt++) {
        await new Promise((resolve) => setTimeout(resolve, confirmInterval))

        const latest = await load()
        if (latest === null) {
          return
        }

        if (!latest.some((booking) => booking.id === id && booking.status === 'pending_payment')) {
          return
        }
      }
    },
    [load],
  )

  useEffect(() => {
    void load()
  }, [load, version])

  return (
    <section className="card">
      <h2>Your bookings</h2>

      {error !== null ? null : bookings === null ? (
        <p className="hint">Loading…</p>
      ) : bookings.length === 0 ? (
        <p className="lede">Nothing booked yet.</p>
      ) : (
        <ul className="bookings">
          {bookings.map((booking) => (
            <li key={booking.id}>
              <span className="booking-when">
                {formatWhen(booking.starts_at)} &ndash; {formatEnd(booking.ends_at)}
              </span>
              {/* A held slot is not a booking, and a list that does not say so
                  reads as though it were paid for. */}
              {booking.status === 'pending_payment' && (
                <PendingBooking
                  booking={booking}
                  onPaid={() => void awaitConfirmation(booking.id)}
                  onGone={() => void load()}
                  onUnauthorized={onUnauthorized}
                />
              )}
              <span className="booking-where">
                {booking.court.name} at {booking.venue.name}, {booking.venue.city}
              </span>
            </li>
          ))}
        </ul>
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </section>
  )
}

function formatWhen(startsAt: string): string {
  const at = new Date(startsAt)
  if (Number.isNaN(at.getTime())) {
    return startsAt
  }

  return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}

// The end is on the same day as the start, so only the time is worth repeating.
function formatEnd(endsAt: string): string {
  const at = new Date(endsAt)
  if (Number.isNaN(at.getTime())) {
    return endsAt
  }

  return at.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}
