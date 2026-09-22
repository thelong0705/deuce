/// <reference types="vite/client" />

interface ImportMetaEnv {
  // Stripe's publishable key. Safe in the bundle by design — it can only
  // start a payment, never move money.
  readonly VITE_STRIPE_PUBLISHABLE_KEY?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
