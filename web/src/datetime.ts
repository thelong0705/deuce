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
