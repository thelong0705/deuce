import { useState } from 'react'
import type { FormEvent } from 'react'

import { CitySelect } from './CitySelect'
import { CourtResult } from './CourtResult'
import { ApiError, searchCourts } from './api'
import type { CourtSearchResult } from './api'
import { addDays, formatDay, isoDate } from './datetime'
import { useCities } from './useCities'

// A court can be booked from today up to two weeks out.
const horizonDays = 14

// anyHour is the empty selection: the search then covers the whole day.
const anyHour = ''

type Props = {
  onBooked: () => void
  onUnauthorized: () => void
}

export function Browse({ onBooked, onUnauthorized }: Props) {
  const { cities, error: citiesError } = useCities(onUnauthorized)
  const [city, setCity] = useState('')
  const [date, setDate] = useState(isoDate(new Date()))
  const [fromHour, setFromHour] = useState(anyHour)
  const [toHour, setToHour] = useState(anyHour)
  // null means nothing has been searched for yet, which is not the same as a
  // search that found nothing.
  const [results, setResults] = useState<CourtSearchResult[] | null>(null)
  // What the results are for, kept apart from the form: editing the date
  // without searching again must not relabel the slots underneath.
  const [searched, setSearched] = useState<{ city: string; date: string } | null>(null)
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function runSearch() {
    setSearching(true)
    setError(null)
    try {
      setResults(
        await searchCourts(
          city,
          date,
          fromHour === anyHour ? null : Number(fromHour),
          toHour === anyHour ? null : Number(toHour),
        ),
      )
      setSearched({ city, date })
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      // Leave the results alone: an error is not the same as finding nothing,
      // and saying both at once reads as nonsense.
      setError(err instanceof ApiError ? err.message : 'Could not search courts.')
    } finally {
      setSearching(false)
    }
  }

  async function handleSearch(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (city === '') {
      setError('Choose a city')
      return
    }

    await runSearch()
  }

  return (
    <section className="card">
      <h2>Book a court</h2>
      <p className="lede">Find a court that is free when you want to play.</p>

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

        <label htmlFor="browse-date">
          Date
          <input
            id="browse-date"
            type="date"
            value={date}
            min={isoDate(new Date())}
            max={isoDate(addDays(new Date(), horizonDays))}
            onChange={(e) => setDate(e.target.value)}
          />
        </label>

        <div className="hour-range">
          <label htmlFor="browse-from">
            From
            <select id="browse-from" value={fromHour} onChange={(e) => setFromHour(e.target.value)}>
              <option value={anyHour}>Any time</option>
              {hours(0, 23).map((hour) => (
                <option key={hour} value={hour}>
                  {label(hour)}
                </option>
              ))}
            </select>
          </label>

          <label htmlFor="browse-to">
            Until
            <select id="browse-to" value={toHour} onChange={(e) => setToHour(e.target.value)}>
              <option value={anyHour}>Any time</option>
              {hours(1, 24).map((hour) => (
                <option key={hour} value={hour}>
                  {label(hour)}
                </option>
              ))}
            </select>
          </label>
        </div>

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
        results !== null &&
        searched !== null &&
        (results.length === 0 ? (
          <p className="lede">
            Nothing free in {searched.city} on {formatDay(searched.date)}. Try another day or a
            wider window.
          </p>
        ) : (
          <ul className="court-results">
            <li className="results-day">Free on {formatDay(searched.date)}</li>
            {results.map((result) => (
              <CourtResult
                key={result.court.id}
                result={result}
                onBooked={onBooked}
                onUnauthorized={onUnauthorized}
                onTaken={() => void runSearch()}
              />
            ))}
          </ul>
        ))}
    </section>
  )
}

function hours(first: number, last: number): number[] {
  return Array.from({ length: last - first + 1 }, (_, i) => first + i)
}

// The hours are the venue's own, so they are shown as plain clock times rather
// than converted to wherever the browser happens to be.
function label(hour: number): string {
  return `${String(hour).padStart(2, '0')}:00`
}
