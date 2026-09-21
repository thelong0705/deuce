import { useState } from 'react'

import { CourtForm, formatHour } from './CourtForm'
import type { Court, Venue } from './api'

type Props = {
  venue: Venue
  onUnauthorized: () => void
}

export function VenueRow({ venue, onUnauthorized }: Props) {
  // Courts added in this session. There is no endpoint to read them back, so
  // this list starts empty on every reload — see the note in web/README.md.
  const [courts, setCourts] = useState<Court[]>([])
  const [adding, setAdding] = useState(false)

  return (
    <li>
      <span className="venue-name">{venue.name}</span>
      {!venue.is_active && <span className="badge">Inactive</span>}
      <span className="venue-where">
        {venue.city} &middot; {venue.address} &middot; {venue.timezone}
      </span>

      {courts.length > 0 && (
        <ul className="courts">
          {courts.map((court) => (
            <li key={court.id}>
              <span>{court.name}</span>
              <span className="court-detail">
                {formatHour(court.open_hour)}&ndash;{formatHour(court.close_hour)} &middot;{' '}
                {court.price_per_hour}/hour
              </span>
            </li>
          ))}
        </ul>
      )}

      {adding ? (
        <>
          <CourtForm
            venueID={venue.id}
            onCreated={(court) => {
              setCourts((prev) => [...prev, court])
              setAdding(false)
            }}
            onUnauthorized={onUnauthorized}
          />
          <button type="button" className="secondary quiet" onClick={() => setAdding(false)}>
            Cancel
          </button>
        </>
      ) : (
        <button type="button" className="link" onClick={() => setAdding(true)}>
          Add a court
        </button>
      )}
    </li>
  )
}
