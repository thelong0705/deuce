import { countryByISO } from './countries'
import { toE164 } from './validation'
import type {
  CourtInput,
  Currency,
  LoginInput,
  Role,
  SignupInput,
  VenueInput,
} from './validation'

// The shape /users and /me both return.
export type User = {
  id: string
  email: string
  display_name: string
  phone_number: string
  role: Role
  created_at: string
}

export type CreatedUser = User

export type Session = {
  user_id: string
  expires_at: string
}

export class ApiError extends Error {
  readonly status: number
  // code is the server's machine-readable error code, e.g. "invalid_credentials".
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

export async function signup(input: SignupInput): Promise<CreatedUser> {
  const response = await request('/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: input.email.trim(),
      password: input.password,
      phone_number: toE164(countryByISO(input.countryISO).dial, input.phoneNumber),
      role: input.role,
    }),
  })

  return (await response.json()) as CreatedUser
}

export async function login(input: LoginInput): Promise<Session> {
  const response = await request('/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      email: input.email.trim(),
      password: input.password,
    }),
  })

  // The token is only in the httpOnly cookie; the body carries nothing secret.
  return (await response.json()) as Session
}

export async function logout(): Promise<void> {
  await request('/sessions', { method: 'DELETE' })
}

// me confirms the session and reports who it belongs to. The stored hint is
// neither proof the session is live nor a source of the role.
export async function me(): Promise<User> {
  const response = await request('/me', { method: 'GET' })

  return (await response.json()) as User
}

export type Venue = {
  id: string
  owner_id: string
  name: string
  city: string
  address: string
  timezone: string
  is_active: boolean
  created_at: string
}

// The owner is whoever the session cookie belongs to, so neither call names it.
export async function createVenue(input: VenueInput): Promise<Venue> {
  const response = await request('/venues', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: input.name.trim(),
      city: input.city.trim(),
      address: input.address.trim(),
    }),
  })

  return (await response.json()) as Venue
}

// The set is fixed and comes from the server, so the client offers a choice
// rather than a free-text field.
export type City = {
  name: string
  timezone: string
}

export async function listCities(): Promise<City[]> {
  const response = await request('/cities', { method: 'GET' })

  const body = (await response.json()) as { cities?: City[] }
  return body.cities ?? []
}

export async function listVenues(): Promise<Venue[]> {
  const response = await request('/venues', {
    method: 'GET',
  })

  const body = (await response.json()) as { venues?: Venue[] }
  return body.venues ?? []
}

export type Court = {
  id: string
  venue_id: string
  name: string
  open_hour: number
  close_hour: number
  price_per_hour: number
  currency: Currency
  is_active: boolean
  created_at: string
}

// The hours and price are numbers on the wire; the form holds them as text.
export async function createCourt(venueID: string, input: CourtInput): Promise<Court> {
  const response = await request(`/venues/${encodeURIComponent(venueID)}/courts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: input.name.trim(),
      open_hour: Number(input.openHour),
      close_hour: Number(input.closeHour),
      price_per_hour: Number(input.pricePerHour),
      currency: input.currency,
    }),
  })

  return (await response.json()) as Court
}

// --- Booking -------------------------------------------------------------
//
// Only createBooking is implemented. The rest are the contract this UI is
// written against; web/README.md spells it out. Until they land, those calls
// come back 404 and the screens say so.

// searchVenues browses every owner's venues; GET /venues is the caller's own.
export async function searchVenues(city: string): Promise<Venue[]> {
  const response = await request(`/venues/search?city=${encodeURIComponent(city.trim())}`, {
    method: 'GET',
  })

  const body = (await response.json()) as { venues?: Venue[] }
  return body.venues ?? []
}

export async function listCourts(venueID: string): Promise<Court[]> {
  const response = await request(`/venues/${encodeURIComponent(venueID)}/courts`, {
    method: 'GET',
  })

  const body = (await response.json()) as { courts?: Court[] }
  return body.courts ?? []
}

// Both instants come from the server and go back untouched, so which timezone
// an hour belongs to, and how long a slot runs, stay the server's to know.
export type Slot = {
  starts_at: string
  ends_at: string
  available: boolean
}

export async function listAvailability(courtID: string, date: string): Promise<Slot[]> {
  const response = await request(
    `/courts/${encodeURIComponent(courtID)}/availability?date=${encodeURIComponent(date)}`,
    { method: 'GET' },
  )

  const body = (await response.json()) as { slots?: Slot[] }
  return body.slots ?? []
}

// CourtSearchResult is a court that still has room, with the venue it stands
// at: a court called "Court 1" says nothing on its own.
export type CourtSearchResult = {
  court: Court
  venue: Venue
  slots: Slot[]
}

// An absent hour is the whole day, so the window is only sent when it narrows
// something.
export async function searchCourts(
  city: string,
  date: string,
  fromHour: number | null,
  toHour: number | null,
): Promise<CourtSearchResult[]> {
  const params = new URLSearchParams({ city: city.trim(), date })
  if (fromHour !== null) {
    params.set('from_hour', String(fromHour))
  }
  if (toHour !== null) {
    params.set('to_hour', String(toHour))
  }

  const response = await request(`/courts/search?${params.toString()}`, { method: 'GET' })

  const body = (await response.json()) as { courts?: CourtSearchResult[] }
  return body.courts ?? []
}

// Booking is what POST returns: the row, and nothing about the court beyond
// its id.
// pending_payment holds the slot; it is only a booking once Stripe says the
// payment went through.
export type BookingStatus = 'pending_payment' | 'confirmed'

export type Booking = {
  id: string
  court_id: string
  player_id: string
  starts_at: string
  ends_at: string
  status: BookingStatus
  // What the slot cost when it was held, in the smallest unit of the court's
  // currency.
  amount: number | null
  // When an unpaid slot goes back. Null once it is paid for.
  hold_expires_at: string | null
  created_at: string
}

// client_secret is sent once, on the response that takes the slot. It is not
// stored and cannot be fetched again.
export type HeldBooking = Booking & {
  client_secret: string
}

// This holds the slot rather than booking it: pay with the client secret, and
// Stripe tells the server the result, not this code.
export async function createBooking(courtID: string, startsAt: string): Promise<HeldBooking> {
  const response = await request(`/courts/${encodeURIComponent(courtID)}/bookings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ starts_at: startsAt }),
  })

  return (await response.json()) as HeldBooking
}

// A listed booking carries the court and venue names; a list of court ids
// tells a player nothing.
export type BookingListItem = Booking & {
  court: { name: string; price_per_hour: number; currency: Currency }
  venue: { id: string; name: string; city: string }
}

export async function listBookings(): Promise<BookingListItem[]> {
  const response = await request('/bookings', { method: 'GET' })

  const body = (await response.json()) as { bookings?: BookingListItem[] }
  return body.bookings ?? []
}

async function request(path: string, init: RequestInit): Promise<Response> {
  let response: Response
  try {
    // Default, but the session cookie makes it load-bearing.
    response = await fetch(path, { credentials: 'same-origin', ...init })
  } catch {
    throw new ApiError(0, 'unreachable', 'Could not reach the server. Is it running?')
  }

  if (!response.ok) {
    throw await apiError(response)
  }

  return response
}

async function apiError(response: Response): Promise<ApiError> {
  try {
    const body: unknown = await response.json()
    if (typeof body === 'object' && body !== null) {
      const { code, error } = body as { code?: unknown; error?: unknown }
      if (typeof error === 'string') {
        return new ApiError(response.status, typeof code === 'string' ? code : '', error)
      }
    }
  } catch {
    // Not JSON; fall through to the generic message.
  }

  if (response.status === 404) {
    return new ApiError(404, '', 'That endpoint does not exist yet.')
  }

  return new ApiError(response.status, '', `Request failed (${response.status})`)
}
