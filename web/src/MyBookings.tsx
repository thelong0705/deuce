import { useCallback, useEffect, useState } from 'react'

import { ApiError, listBookings } from './api'
import type { BookingListItem } from './api'

type Props = {
  // version changes when a booking is made, which reloads the list.
  version: number
  onUnauthorized: () => void
}

export function MyBookings({ version, onUnauthorized }: Props) {
  const [bookings, setBookings] = useState<BookingListItem[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      setBookings(await listBookings())
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      // Not the same as having nothing booked, so do not say that.
      setError(err instanceof ApiError ? err.message : 'Could not load your bookings.')
    }
  }, [onUnauthorized])

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
