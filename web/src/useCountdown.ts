import { useEffect, useState } from 'react'

// useCountdown reports the milliseconds left until `until`, ticking every
// second and stopping at zero. A null deadline means nothing to count.
//
// It reads the clock rather than decrementing a stored number, so a tab that
// was asleep comes back with the right answer instead of however many ticks
// it managed to run.
export function useCountdown(until: string | null): number | null {
  const [remaining, setRemaining] = useState(() => msUntil(until))

  useEffect(() => {
    if (until === null) {
      setRemaining(null)
      return
    }

    setRemaining(msUntil(until))

    const id = setInterval(() => {
      const left = msUntil(until)
      setRemaining(left)
      if (left !== null && left <= 0) {
        clearInterval(id)
      }
    }, 1000)

    return () => clearInterval(id)
  }, [until])

  return remaining
}

function msUntil(until: string | null): number | null {
  if (until === null) {
    return null
  }

  const at = Date.parse(until)
  if (Number.isNaN(at)) {
    return null
  }

  return Math.max(0, at - Date.now())
}

// formatCountdown is mm:ss, which is the only shape a fifteen-minute hold
// needs.
export function formatCountdown(ms: number): string {
  const total = Math.ceil(ms / 1000)
  const minutes = Math.floor(total / 60)
  const seconds = total % 60

  return `${minutes}:${String(seconds).padStart(2, '0')}`
}
