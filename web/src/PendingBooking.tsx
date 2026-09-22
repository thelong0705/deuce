import { useState } from 'react'

import { Elements } from '@stripe/react-stripe-js'

import { PaymentForm } from './PaymentForm'
import { ApiError, resumePayment } from './api'
import type { BookingListItem, HeldBooking } from './api'
import { elementsAppearance, stripeConfigured, stripePromise } from './stripe'
import { formatCountdown, useCountdown } from './useCountdown'

type Props = {
  booking: BookingListItem
  // onPaid and onGone both mean the list is out of date, for different
  // reasons; the caller reloads either way.
  onPaid: () => void
  onGone: () => void
  onUnauthorized: () => void
}

export function PendingBooking({ booking, onPaid, onGone, onUnauthorized }: Props) {
  const [held, setHeld] = useState<HeldBooking | null>(null)
  const [opening, setOpening] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const remaining = useCountdown(booking.hold_expires_at)
  const lapsed = remaining !== null && remaining <= 0

  async function openPayment() {
    setOpening(true)
    setError(null)
    try {
      setHeld(await resumePayment(booking.id))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
      } else if (err instanceof ApiError && err.status === 409) {
        // The hold went, or somebody already paid. Either way the list is
        // stale, and its own words are better than a guess.
        setError(err.message)
        onGone()
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Something went wrong.')
      }
    } finally {
      setOpening(false)
    }
  }

  if (lapsed) {
    return <span className="badge">Hold expired</span>
  }

  return (
    <>
      <span className="badge">
        Awaiting payment{remaining === null ? '' : ` · ${formatCountdown(remaining)} left`}
      </span>

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      {held === null ? (
        <button type="button" className="link" onClick={() => void openPayment()} disabled={opening}>
          {opening ? 'Opening…' : payLabel(booking)}
        </button>
      ) : stripeConfigured() ? (
        <div className="payment-panel">
          <Elements
            key={held.client_secret}
            stripe={stripePromise()}
            options={{ clientSecret: held.client_secret, appearance: elementsAppearance() }}
          >
            <PaymentForm
              amount={held.amount}
              currency={booking.court.currency}
              onPaid={onPaid}
              onCancel={() => setHeld(null)}
            />
          </Elements>
        </div>
      ) : (
        <p className="form-error" role="alert">
          Payments are not configured here, so nothing can pay for this hold.
        </p>
      )}
    </>
  )
}

function payLabel(booking: BookingListItem): string {
  return booking.amount === null
    ? 'Pay now'
    : `Pay ${booking.amount.toLocaleString()} ${booking.court.currency}`
}
