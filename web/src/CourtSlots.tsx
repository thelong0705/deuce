import { useCallback, useEffect, useState } from 'react'

import { ApiError, createBooking, listAvailability } from './api'
import type { Court, Slot } from './api'

// A court can be booked from today up to two weeks out.
const horizonDays = 14

type Props = {
  court: Court
  onBooked: () => void
  onUnauthorized: () => void
}

export function CourtSlots({ court, onBooked, onUnauthorized }: Props) {
  const [date, setDate] = useState(isoDate(new Date()))
  const [slots, setSlots] = useState<Slot[] | null>(null)
  const [picked, setPicked] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [booking, setBooking] = useState(false)
  const [booked, setBooked] = useState<string | null>(null)

  const load = useCallback(async () => {
    setSlots(null)
    setPicked(null)
    setError(null)
    try {
      setSlots(await listAvailability(court.id, date))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
        return
      }
      // Not the same as a day with no bookable hours.
      setError(err instanceof ApiError ? err.message : 'Could not load times.')
    }
  }, [court.id, date, onUnauthorized])

  useEffect(() => {
    void load()
  }, [load])

  async function handleBook() {
    if (picked === null) {
      return
    }

    setBooking(true)
    setError(null)
    try {
      const made = await createBooking(court.id, picked)
      setBooked(made.starts_at)
      onBooked()
      // Whatever else changed while the slots were on screen, the booked one
      // is certainly gone now.
      await load()
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
      } else if (err instanceof ApiError && err.code === 'slot_taken') {
        // Reload first: load() clears the error, so setting it beforehand
        // would wipe the one explanation of why nothing was booked.
        await load()
        setError('Someone just took that slot.')
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Something went wrong.')
      }
    } finally {
      setBooking(false)
    }
  }

  return (
    <div className="slots-panel">
      <label htmlFor={`date-${court.id}`}>
        Date
        <input
          id={`date-${court.id}`}
          type="date"
          value={date}
          min={isoDate(new Date())}
          max={isoDate(addDays(new Date(), horizonDays))}
          onChange={(e) => {
            setDate(e.target.value)
            setBooked(null)
          }}
        />
      </label>

      {error !== null && slots === null ? null : slots === null ? (
        <p className="hint">Loading times…</p>
      ) : slots.length === 0 ? (
        <p className="hint">No times for this day.</p>
      ) : (
        <ul className="slots">
          {slots.map((slot) => (
            <li key={slot.starts_at}>
              <button
                type="button"
                className="slot"
                disabled={!slot.available}
                aria-pressed={picked === slot.starts_at}
                onClick={() => {
                  setPicked(slot.starts_at)
                  setBooked(null)
                }}
              >
                {formatTime(slot.starts_at)}
              </button>
            </li>
          ))}
        </ul>
      )}

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {booked && (
        <p className="notice" role="status">
          Booked {formatTime(booked)} on {court.name}.
        </p>
      )}

      {picked && (
        <button type="button" onClick={handleBook} disabled={booking}>
          {booking ? 'Booking…' : `Book ${formatTime(picked)} · ${court.price_per_hour}`}
        </button>
      )}
    </div>
  )
}

function isoDate(at: Date): string {
  // toISOString would shift the date across midnight for anyone behind UTC.
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`
}

function addDays(at: Date, days: number): Date {
  const next = new Date(at)
  next.setDate(next.getDate() + days)
  return next
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

function formatTime(startsAt: string): string {
  const at = new Date(startsAt)
  if (Number.isNaN(at.getTime())) {
    return startsAt
  }

  return at.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}
