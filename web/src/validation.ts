export const MIN_PASSWORD_BYTES = 8
export const MAX_PASSWORD_BYTES = 72

export type Role = 'player' | 'owner'

export type SignupInput = {
  email: string
  password: string
  // countryISO picks the dialling code; phoneNumber is the national part as
  // typed. The two are joined into E.164 only when the request goes out.
  countryISO: string
  phoneNumber: string
  role: Role
}

export type LoginInput = {
  email: string
  password: string
}

export type VenueInput = {
  name: string
  city: string
  address: string
}

export type FieldErrors = Partial<Record<keyof SignupInput, string>>

export type LoginFieldErrors = Partial<Record<keyof LoginInput, string>>

export type VenueFieldErrors = Partial<Record<keyof VenueInput, string>>

// Hours and price are held as text so the form owns the raw input; they are
// parsed once, in validateCourt.
export type CourtInput = {
  name: string
  openHour: string
  closeHour: string
  pricePerHour: string
}

export type CourtFieldErrors = Partial<Record<keyof CourtInput, string>>

// Go measures passwords with len(), which counts bytes. A password of accented
// or Vietnamese characters can pass a .length check here and still be rejected
// by the server, so count bytes the same way it does.
export function passwordByteLength(password: string): number {
  return new TextEncoder().encode(password).length
}

const emailPattern = /^[^@\s]+@[^@\s]+\.[^@\s]+$/

// nationalDigits keeps only the digits and drops one leading trunk zero, the
// 0 in 0901234567 that a Vietnamese number is written with locally and never
// carries internationally.
export function nationalDigits(phoneNumber: string): string {
  return phoneNumber.replace(/\D/g, '').replace(/^0/, '')
}

// toE164 is the only place the country code and the typed number are joined,
// and the only shape the server is ever sent.
export function toE164(dial: string, phoneNumber: string): string {
  return `+${dial}${nationalDigits(phoneNumber)}`
}

export function validate(input: SignupInput): FieldErrors {
  const errors: FieldErrors = {}

  if (!emailPattern.test(input.email.trim())) {
    errors.email = 'Enter a valid email address'
  }

  const bytes = passwordByteLength(input.password)
  if (bytes < MIN_PASSWORD_BYTES) {
    errors.password = `Must be at least ${MIN_PASSWORD_BYTES} characters`
  } else if (bytes > MAX_PASSWORD_BYTES) {
    errors.password = `Too long — ${MAX_PASSWORD_BYTES} characters maximum`
  }

  const digits = nationalDigits(input.phoneNumber)
  if (input.phoneNumber.trim() === '') {
    errors.phoneNumber = 'Phone number is required'
  } else if (/[^\d\s\-().]/.test(input.phoneNumber)) {
    errors.phoneNumber = 'Digits only — the country code is already set'
  } else if (digits.length < 4 || digits.length > 14) {
    // E.164 allows fifteen digits including the country code, so the national
    // part can never be longer than fourteen.
    errors.phoneNumber = 'That does not look like a phone number'
  }

  if (input.role !== 'player' && input.role !== 'owner') {
    errors.role = 'Choose an account type'
  }

  return errors
}

// Signing in only checks that something was filled in. Holding an existing
// password to the signup rules would reject accounts the server still accepts,
// and saying "too short" on a sign-in form is a hint about a password nobody
// has proven they own.
export function validateLogin(input: LoginInput): LoginFieldErrors {
  const errors: LoginFieldErrors = {}

  if (!emailPattern.test(input.email.trim())) {
    errors.email = 'Enter a valid email address'
  }

  if (input.password === '') {
    errors.password = 'Enter your password'
  }

  return errors
}

// Mirrors CreateVenueInput.Validate, which trims before checking. It reports
// only the first broken rule; this reports all of them so the form can mark
// every empty field at once.
export function validateVenue(input: VenueInput): VenueFieldErrors {
  const errors: VenueFieldErrors = {}

  if (input.name.trim() === '') {
    errors.name = 'Venue name is required'
  }

  if (input.city.trim() === '') {
    errors.city = 'City is required'
  }

  if (input.address.trim() === '') {
    errors.address = 'Address is required'
  }

  return errors
}

// Mirrors CreateCourtInput.Validate. The hour selects already keep both values
// in range, so the only ordering rule left to state is open before close.
export function validateCourt(input: CourtInput): CourtFieldErrors {
  const errors: CourtFieldErrors = {}

  if (input.name.trim() === '') {
    errors.name = 'Court name is required'
  }

  const open = Number(input.openHour)
  const close = Number(input.closeHour)
  if (open >= close) {
    errors.closeHour = 'Closing hour must be after opening hour'
  }

  const price = Number(input.pricePerHour)
  if (input.pricePerHour.trim() === '' || !Number.isInteger(price)) {
    errors.pricePerHour = 'Enter a whole number'
  } else if (price < 0) {
    errors.pricePerHour = 'Price cannot be negative'
  }

  return errors
}
