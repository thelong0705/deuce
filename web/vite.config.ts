import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Proxying keeps the API same-origin with the app, so the session cookie
    // is sent on every request without CORS or credentials wrangling.
    proxy: Object.fromEntries(
      ['/users', '/sessions'].map((path) => [
        path,
        { target: process.env.API_URL ?? 'http://localhost:8080', changeOrigin: true },
      ]),
    ),
  },
})
