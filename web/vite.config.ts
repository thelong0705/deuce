import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Configuration lives in .env.development and .env.production, which Vite
// reads by mode. Both are committed: the only value in them is the Stripe
// publishable key, which ships in the bundle regardless.
export default defineConfig({
  plugins: [react()],
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
})
