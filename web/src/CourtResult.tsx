import { useState } from 'react'

import { ApiError, createBooking } from './api'
import type { CourtSearchResult, Slot } from './api'
import { formatWindow } from './datetime'

type Props = {
  result: CourtSearchResult
  onBooked: () => void
  onUnauthorized: () => void
  // onTaken reruns the search, since a slot somebody else took is not the only
  // thing that may have changed.
  onTaken: () => void
}

export function CourtResult({ result, onBooked, onUnauthorized, onTaken }: Props) {
  const { court, venue, slots } = result

  const [booking, setBooking] = useState<string | null>(null)
  const [booked, setBooked] = useState<Slot | null>(null)
  const [error, setError] = useState<string | null>(null)

  async function book(slot: Slot) {
    setBooking(slot.starts_at)
    setError(null)
    try {
      await createBooking(court.id, slot.starts_at)
      setBooked(slot)
      onBooked()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
      } else if (err instanceof ApiError && err.code === 'slot_taken') {
        setError('Someone just took that slot.')
        onTaken()
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Something went wrong.')
      }
    } finally {
      setBooking(null)
    }
  }

  return (
    <li className="court-result">
      <div className="court-result-head">
        <span className="venue-name">{venue.name}</span>
        <span className="venue-where">
          {court.name} &middot; {venue.address} &middot; {court.price_per_hour}
        </span>
      </div>

      <ul className="slots">
        {slots.map((slot) => (
          <li key={slot.starts_at}>
            <button
              type="button"
              className="slot"
              disabled={booking !== null || booked !== null}
              onClick={() => void book(slot)}
            >
              {booking === slot.starts_at ? 'Booking…' : formatWindow(slot.starts_at, slot.ends_at)}
            </button>
          </li>
        ))}
      </ul>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {booked && (
        <p className="notice" role="status">
          Booked {formatWindow(booked.starts_at, booked.ends_at)} on {court.name}.
        </p>
      )}
    </li>
  )
}
