import { useState } from 'react'
import type { FormEvent } from 'react'

import { BrowseVenue } from './BrowseVenue'
import { CitySelect } from './CitySelect'
import { ApiError, searchVenues } from './api'
import { useCities } from './useCities'
import type { Venue } from './api'

type Props = {
  onBooked: () => void
  onUnauthorized: () => void
}

export function Browse({ onBooked, onUnauthorized }: Props) {
  const { cities, error: citiesError } = useCities(onUnauthorized)
  const [city, setCity] = useState('')
  // null means nothing has been searched for yet, which is not the same as a
  // city with no venues.
  const [venues, setVenues] = useState<Venue[] | null>(null)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (city === '') {
      setError('Choose a city')
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
        <CitySelect
          id="browse-city"
          label="City"
          value={city}
          cities={cities}
          error={citiesError ?? undefined}
          onChange={(next) => {
            setCity(next)
            setError(null)
          }}
        />

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
