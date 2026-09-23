import { fileURLToPath } from 'node:url'

import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  // The publishable key lives in the repo root .env beside the server's Stripe
  // keys, rather than being duplicated here under a VITE_ prefix. Reading it
  // from there keeps one name for one secret-adjacent value.
  const root = fileURLToPath(new URL('..', import.meta.url))
  const env = loadEnv(mode, root, '')

  // process.env first, so a Docker build arg wins over a developer's .env.
  const value = (key: string) => process.env[key] ?? env[key] ?? ''

  return {
    plugins: [react()],
    define: {
      'import.meta.env.VITE_STRIPE_PUBLISHABLE_KEY': JSON.stringify(
        value('STRIPE_PUBLISHABLE_KEY'),
      ),
      // Empty means same-origin, which is what the dev proxy below gives us.
      'import.meta.env.VITE_API_BASE_URL': JSON.stringify(value('API_BASE_URL')),
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
