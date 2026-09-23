// isoDate is the YYYY-MM-DD the API takes. toISOString would shift the date
// across midnight for anyone behind UTC.
export function isoDate(at: Date): string {
  return `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}`
}

export function addDays(at: Date, days: number): Date {
  const next = new Date(at)
  next.setDate(next.getDate() + days)
  return next
}

export function formatWindow(startsAt: string, endsAt: string): string {
  return `${formatTime(startsAt)}–${formatTime(endsAt)}`
}

// formatDay takes the YYYY-MM-DD that was searched for. Handing that string to
// new Date reads it as UTC midnight, which is the day before for anyone behind
// UTC, so the parts are used as a local date instead.
export function formatDay(iso: string): string {
  const [year, month, day] = iso.split('-').map(Number)
  const at = new Date(year, month - 1, day)
  if (Number.isNaN(at.getTime())) {
    return iso
  }

  return at.toLocaleDateString(undefined, {
    weekday: 'long',
    day: 'numeric',
    month: 'long',
    year: 'numeric',
  })
}

function formatTime(at: string): string {
  const when = new Date(at)
  if (Number.isNaN(when.getTime())) {
    return at
  }

  return when.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}
