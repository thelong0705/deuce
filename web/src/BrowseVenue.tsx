import { useState } from 'react'

import { CourtSlots } from './CourtSlots'
import { ApiError, listCourts } from './api'
import type { Court, Venue } from './api'

type Props = {
  venue: Venue
  onBooked: () => void
  onUnauthorized: () => void
}

export function BrowseVenue({ venue, onBooked, onUnauthorized }: Props) {
  const [courts, setCourts] = useState<Court[] | null>(null)
  const [open, setOpen] = useState(false)
  const [picked, setPicked] = useState<Court | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function expand() {
    setOpen(true)
    if (courts !== null) {
      return
    }

    setError(null)
    try {
      setCourts(await listCourts(venue.id))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      // Not the same as a venue with no courts.
      setError(err instanceof ApiError ? err.message : 'Could not load courts.')
    }
  }

  return (
    <li>
      <span className="venue-name">{venue.name}</span>
      <span className="venue-where">
        {venue.city} &middot; {venue.address}
      </span>

      {!open ? (
        <button type="button" className="link" onClick={() => void expand()}>
          See courts
        </button>
      ) : error !== null ? null : courts === null ? (
        <p className="hint">Loading courts…</p>
      ) : courts.length === 0 ? (
        <p className="hint">This venue has no courts yet.</p>
      ) : (
        <ul className="courts">
          {courts.map((court) => (
            <li key={court.id}>
              <button
                type="button"
                className="link"
                aria-pressed={picked?.id === court.id}
                onClick={() => setPicked(picked?.id === court.id ? null : court)}
              >
                {court.name} &middot; {court.price_per_hour}/hour
              </button>

              {picked?.id === court.id && (
                <CourtSlots court={court} onBooked={onBooked} onUnauthorized={onUnauthorized} />
              )}
            </li>
          ))}
        </ul>
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
    </li>
  )
}
