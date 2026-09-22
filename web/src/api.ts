import { countryByISO } from './countries'
import { toE164 } from './validation'
import type { CourtInput, LoginInput, Role, SignupInput, VenueInput } from './validation'

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

  // The session token is only ever in the httpOnly cookie the server set; the
  // body carries nothing secret.
  return (await response.json()) as Session
}

export async function logout(): Promise<void> {
  await request('/sessions', { method: 'DELETE' })
}

// me confirms the session with the server and reports who it belongs to. The
// stored hint cannot answer either question: it is not proof the session is
// still live, and it does not carry the role.
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
      timezone: input.timezone,
    }),
  })

  return (await response.json()) as Venue
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
  is_active: boolean
  created_at: string
}

// The hours and the price are numbers on the wire; the form holds them as text
// until validation has had a look.
export async function createCourt(venueID: string, input: CourtInput): Promise<Court> {
  const response = await request(`/venues/${encodeURIComponent(venueID)}/courts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: input.name.trim(),
      open_hour: Number(input.openHour),
      close_hour: Number(input.closeHour),
      price_per_hour: Number(input.pricePerHour),
    }),
  })

  return (await response.json()) as Court
}

// --- Booking -------------------------------------------------------------
//
// Only createBooking is implemented. The rest are the contract this UI is
// written against; web/README.md spells it out. Until they land, those calls
// come back 404 and the screens say so.

// searchVenues browses every owner's venues, unlike GET /venues, which only
// ever returns the caller's own.
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

export type Slot = {
  // starts_at is an instant, and the client never builds one: it books by
  // echoing back a value the server offered. That keeps the whole question of
  // which timezone an hour belongs to on the server, where the venue is.
  starts_at: string
  // ends_at comes from the server too, so how long a slot runs stays one
  // fact in one place rather than a duration duplicated here.
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

// Booking is what POST returns: the row, and nothing about the court beyond
// its id. The court is already in the URL, so the server has no reason to
// repeat it.
export type Booking = {
  id: string
  court_id: string
  player_id: string
  starts_at: string
  ends_at: string
  created_at: string
}

// This one is implemented. The court is named by the path, so the body is
// only the hour.
export async function createBooking(courtID: string, startsAt: string): Promise<Booking> {
  const response = await request(`/courts/${encodeURIComponent(courtID)}/bookings`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ starts_at: startsAt }),
  })

  return (await response.json()) as Booking
}

// A listed booking carries the court and venue names, because a list of court
// ids tells a player nothing. Resolving them client side would be one request
// per row, so the server is the place to join.
export type BookingListItem = Booking & {
  court: { name: string; price_per_hour: number }
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
    // same-origin is the default, but the session cookie makes it load-bearing
    // enough to say out loud.
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
