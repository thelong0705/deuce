import { useState } from 'react'

import { Elements } from '@stripe/react-stripe-js'

import { PaymentForm } from './PaymentForm'
import { ApiError, createBooking } from './api'
import type { CourtSearchResult, HeldBooking, Slot } from './api'
import { formatWindow } from './datetime'
import { formatCountdown, useCountdown } from './useCountdown'
import { elementsAppearance, stripeConfigured, stripePromise } from './stripe'

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
  // held is a slot taken but not paid for; the payment form stands on it.
  const [held, setHeld] = useState<{ booking: HeldBooking; slot: Slot } | null>(null)
  const [booked, setBooked] = useState<Slot | null>(null)
  const [error, setError] = useState<string | null>(null)

  const remaining = useCountdown(held?.booking.hold_expires_at ?? null)
  // Nothing can be paid for after this: the slot is already back, and the
  // server would refuse the payment anyway.
  const lapsed = remaining !== null && remaining <= 0

  async function book(slot: Slot) {
    setBooking(slot.starts_at)
    setError(null)
    try {
      setHeld({ booking: await createBooking(court.id, slot.starts_at), slot })
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
          {court.name} &middot; {venue.address} &middot; {court.price_per_hour} {court.currency}
        </span>
      </div>

      <ul className="slots">
        {slots.map((slot) => (
          <li key={slot.starts_at}>
            <button
              type="button"
              className="slot"
              disabled={booking !== null || held !== null || booked !== null}
              onClick={() => void book(slot)}
            >
              {booking === slot.starts_at ? 'Holding…' : formatWindow(slot.starts_at, slot.ends_at)}
            </button>
          </li>
        ))}
      </ul>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {held && (
        <div className="payment-panel">
          <p className="lede">
            Holding {formatWindow(held.slot.starts_at, held.slot.ends_at)} on {court.name}.
          </p>

          {lapsed ? (
            <p className="form-error" role="alert">
              That hold ran out and the slot has gone back. Search again to pick another time.
            </p>
          ) : (
            <p className="countdown" role="timer">
              {remaining === null ? (
                'Pay to keep it.'
              ) : (
                <>
                  <strong>{formatCountdown(remaining)}</strong> left to pay
                </>
              )}
            </p>
          )}

          {lapsed ? null : stripeConfigured() ? (
            <Elements
              // Remounting per hold is deliberate: Elements takes the client
              // secret once, and this is a different payment each time.
              key={held.booking.client_secret}
              stripe={stripePromise()}
              options={{ clientSecret: held.booking.client_secret, appearance: elementsAppearance() }}
            >
              <PaymentForm
                amount={held.booking.amount}
                currency={court.currency}
                onPaid={() => {
                  setBooked(held.slot)
                  setHeld(null)
                  onBooked()
                }}
                onCancel={() => setHeld(null)}
              />
            </Elements>
          ) : (
            <p className="form-error" role="alert">
              Payments are not configured here: VITE_STRIPE_PUBLISHABLE_KEY is unset. The slot is
              held, but nothing can pay for it.
            </p>
          )}
        </div>
      )}

      {booked && (
        <p className="notice" role="status">
          Paid and booked {formatWindow(booked.starts_at, booked.ends_at)} on {court.name}.
        </p>
      )}
    </li>
  )
}
