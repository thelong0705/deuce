import { useEffect, useState } from 'react'

import { ApiError, listCities } from './api'
import type { City } from './api'

type Cities = {
  // null until the request settles, which is not the same as none.
  cities: City[] | null
  error: string | null
}

export function useCities(onUnauthorized: () => void): Cities {
  const [cities, setCities] = useState<City[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    void (async () => {
      try {
        const loaded = await listCities()
        if (!cancelled) {
          setCities(loaded)
        }
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          onUnauthorized()
        } else if (!cancelled) {
          setError(err instanceof ApiError ? err.message : 'Could not load cities.')
        }
      }
    })()

    return () => {
      cancelled = true
    }
  }, [onUnauthorized])

  return { cities, error }
}
