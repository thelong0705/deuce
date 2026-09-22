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
      onPaid()
      return
    }

    // processing, or awaiting an action Stripe handled elsewhere. The webhook
    // is what confirms the booking either way, so this only has to stop
    // pretending the payment failed.
    setError('Payment is still going through. Your bookings will update when it clears.')
    setPaying(false)
  }

  return (
    <form className="payment-form" onSubmit={handleSubmit}>
      <PaymentElement />

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
