// The server stores an IANA timezone per venue and reads the courts' opening
// hours in it, so the form has to offer real zone names rather than offsets.

// Used when the browser will not enumerate zones. Enough to cover the places
// deuce is likely to run, and anyone elsewhere still gets their own zone from
// browserTimezone below.
const fallback = [
  'Asia/Ho_Chi_Minh',
  'Asia/Bangkok',
  'Asia/Singapore',
  'Asia/Kuala_Lumpur',
  'Asia/Jakarta',
  'Asia/Manila',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Shanghai',
  'Australia/Sydney',
  'Europe/London',
  'Europe/Paris',
  'America/New_York',
  'America/Los_Angeles',
  'UTC',
]

// browserTimezone is the best default there is: an owner is nearly always
// registering a venue in the zone they are sitting in.
export function browserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || fallback[0]
  } catch {
    return fallback[0]
  }
}

// Intl.supportedValuesOf is ES2024 and this project targets ES2022, so it is
// not in the type definitions yet even though every browser we care about has
// it. The cast is narrowed to the one call.
type IntlWithSupportedValues = {
  supportedValuesOf?: (key: 'timeZone') => string[]
}

export function supportedTimezones(): string[] {
  const current = browserTimezone()

  let zones: string[] = []
  try {
    zones = (Intl as IntlWithSupportedValues).supportedValuesOf?.('timeZone') ?? []
  } catch {
    zones = []
  }

  if (zones.length === 0) {
    zones = fallback
  }

  // A zone the browser reports but will not list still has to be selectable,
  // or the default would silently fall off the list.
  return zones.includes(current) ? zones : [current, ...zones]
}
