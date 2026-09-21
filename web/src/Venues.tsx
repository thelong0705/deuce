import { useCallback, useEffect, useState } from 'react'

import { VenueForm } from './VenueForm'
import { VenueRow } from './VenueRow'
import { ApiError, listVenues } from './api'
import type { Venue } from './api'

type Props = {
  onUnauthorized: () => void
}

export function Venues({ onUnauthorized }: Props) {
  // null means the first load has not finished yet, which is different from
  // an owner with no venues.
  const [venues, setVenues] = useState<Venue[] | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)

  const refresh = useCallback(async () => {
    setLoadError(null)
    try {
      setVenues(await listVenues())
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      setVenues([])
      setLoadError(err instanceof ApiError ? err.message : 'Could not load your venues.')
    }
  }, [onUnauthorized])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return (
    <section className="card">
      <h2>Your venues</h2>

      {venues === null ? (
        <p className="hint">Loading…</p>
      ) : venues.length === 0 ? (
        <p className="lede">No venues yet. Register your first one below.</p>
      ) : (
        <ul className="venues">
          {venues.map((venue) => (
            <VenueRow key={venue.id} venue={venue} onUnauthorized={onUnauthorized} />
          ))}
        </ul>
      )}

      {loadError && (
        <p className="form-error" role="alert">
          {loadError}
        </p>
      )}

      {adding ? (
        <>
          <h3>Register a venue</h3>
          <VenueForm
            onCreated={(venue) => {
              setVenues((prev) => [...(prev ?? []), venue])
              setAdding(false)
            }}
            onUnauthorized={onUnauthorized}
          />
          <button type="button" className="secondary quiet" onClick={() => setAdding(false)}>
            Cancel
          </button>
        </>
      ) : (
        <button type="button" className="secondary" onClick={() => setAdding(true)}>
          Register a venue
        </button>
      )}
    </section>
  )
}
