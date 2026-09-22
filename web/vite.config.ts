import { fileURLToPath } from 'node:url'

import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  // The publishable key lives in the repo root .env beside the server's Stripe
  // keys, rather than being duplicated here under a VITE_ prefix. Reading it
  // from there keeps one name for one secret-adjacent value.
  const root = fileURLToPath(new URL('..', import.meta.url))
  const env = loadEnv(mode, root, '')

  return {
    plugins: [react()],
    define: {
      'import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY': JSON.stringify(
        env.STRIPE_PUBLISHABLE_KEY ?? '',
      ),
    },
    server: {
      port: 5173,
      // Proxying keeps the API same-origin with the app, so the session cookie
      // is sent on every request without CORS or credentials wrangling.
      proxy: Object.fromEntries(
        ['/users', '/sessions', '/venues', '/courts', '/bookings', '/cities', '/me'].map((path) => [
          path,
          { target: process.env.API_URL ?? 'http://localhost:8080', changeOrigin: true },
        ]),
      ),
    },
  }
})
