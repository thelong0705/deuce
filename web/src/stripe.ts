import { loadStripe } from '@stripe/stripe-js'
import type { Appearance, Stripe } from '@stripe/stripe-js'

const publishableKey = import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY ?? ''

// loadStripe fetches a script, so it is called once for the page rather than
// per payment. The promise is created lazily so a build without a key never
// reaches for Stripe at all.
let loading: Promise<Stripe | null> | null = null

export function stripePromise(): Promise<Stripe | null> {
  loading ??= loadStripe(publishableKey)
  return loading
}

// Without a key there is nothing to mount, and saying so beats a blank panel
// and a console error.
export function stripeConfigured(): boolean {
  return publishableKey !== ''
}

// Stripe renders its own iframe, so the page's colour scheme has to be handed
// over rather than inherited.
export function elementsAppearance(): Appearance {
  let dark = false
  try {
    dark = window.matchMedia('(prefers-color-scheme: dark)').matches
  } catch {
    // matchMedia is absent in some test environments; light is the safer miss.
  }

  return { theme: dark ? 'night' : 'stripe' }
}
