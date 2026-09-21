import { useState } from 'react'
import type { FormEvent } from 'react'

import { BrowseVenue } from './BrowseVenue'
import { ApiError, searchVenues } from './api'
import type { Venue } from './api'

type Props = {
  onBooked: () => void
  onUnauthorized: () => void
}

export function Browse({ onBooked, onUnauthorized }: Props) {
  const [city, setCity] = useState('')
  // null means nothing has been searched for yet, which is not the same as a
  // city with no venues.
  const [venues, setVenues] = useState<Venue[] | null>(null)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (city.trim() === '') {
      setError('Enter a city')
      return
    }

    setSearching(true)
    setError(null)
    try {
      setVenues(await searchVenues(city))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      // Leave the results alone: an error is not the same as a city with no
      // venues, and saying both at once reads as nonsense.
      setError(err instanceof ApiError ? err.message : 'Could not search venues.')
    } finally {
      setSearching(false)
    }
  }

  return (
    <section className="card">
      <h2>Book a court</h2>

      <form onSubmit={handleSearch} noValidate>
        <label htmlFor="browse-city">
          City
          <input
            id="browse-city"
            autoComplete="address-level2"
            placeholder="Da Nang"
            value={city}
            onChange={(e) => {
              setCity(e.target.value)
              setError(null)
            }}
          />
        </label>

        <button type="submit" disabled={searching}>
          {searching ? 'Searching…' : 'Search'}
        </button>
      </form>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {error === null &&
        venues !== null &&
        (venues.length === 0 ? (
          <p className="lede">No venues in {city}.</p>
        ) : (
          <ul className="venues">
            {venues.map((venue) => (
              <BrowseVenue
                key={venue.id}
                venue={venue}
                onBooked={onBooked}
                onUnauthorized={onUnauthorized}
              />
            ))}
          </ul>
        ))}
    </section>
  )
}
