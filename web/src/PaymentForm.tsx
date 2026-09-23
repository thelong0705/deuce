import { useState } from 'react'
import type { FormEvent } from 'react'

import { PaymentElement, useElements, useStripe } from '@stripe/react-stripe-js'

type Props = {
  // amount and currency are what the slot was held at, for the button to say.
  amount: number | null
  currency: string
  onPaid: () => void
  onCancel: () => void
}

export function PaymentForm({ amount, currency, onPaid, onCancel }: Props) {
  const stripe = useStripe()
  const elements = useElements()

  const [error, setError] = useState<string | null>(null)
  const [paying, setPaying] = useState(false)
  const [settled, setSettled] = useState(false)

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    if (!stripe || !elements) {
      return
    }

    setPaying(true)
    setError(null)

    // redirect: 'if_required' keeps a card payment on this page. A method that
    // genuinely needs a redirect still gets one, and comes back to the page it
    // left.
    const { error: stripeError, paymentIntent } = await stripe.confirmPayment({
      elements,
      confirmParams: { return_url: window.location.href },
      redirect: 'if_required',
    })

    if (stripeError) {
      // card_error and validation_error are the player's to fix and say so
      // plainly. Anything else is ours, and its message is not for them.
      const kind = stripeError.type
      setError(
        kind === 'card_error' || kind === 'validation_error'
          ? (stripeError.message ?? 'That card was declined.')
          : 'Could not take the payment. Nothing has been charged.',
      )
      setPaying(false)
      return
    }

    if (paymentIntent?.status === 'succeeded') {
      // The card is done, the booking is not: it stays a hold until the webhook
      // lands. Leaving the button mid-payment reads as a page that has hung, so
      // the form gives way to what is actually happening.
      setSettled(true)
      onPaid()
      return
    }

    // processing, or awaiting an action Stripe handled elsewhere. The webhook
    // is what confirms the booking either way, so this only has to stop
    // pretending the payment failed.
    setError('Payment is still going through. Your bookings will update when it clears.')
    setPaying(false)
  }

  // No form once the card has gone through: there is nothing left to submit,
  // and a live Pay button invites paying twice for the same slot.
  if (settled) {
    return <p className="hint">Payment received. Confirming your booking…</p>
  }

  return (
    <form className="payment-form" onSubmit={handleSubmit}>
      {/* Link's inline signup asks for an email, a phone number and a name to
          open a Stripe account. None of that is deuce's to collect at a court
          booking, and the player already has an account here. */}
      <PaymentElement options={{ wallets: { link: 'never' } }} />

      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}

      <button type="submit" disabled={!stripe || paying}>
        {paying ? 'Paying…' : payLabel(amount, currency)}
      </button>

      <button type="button" className="secondary quiet" onClick={onCancel} disabled={paying}>
        Not now
      </button>
    </form>
  )
}

function payLabel(amount: number | null, currency: string): string {
  return amount === null ? 'Pay' : `Pay ${amount.toLocaleString()} ${currency}`
}
