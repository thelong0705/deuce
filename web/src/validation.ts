export const MIN_PASSWORD_BYTES = 8
export const MAX_PASSWORD_BYTES = 72

export type Role = 'player' | 'owner'

export type SignupInput = {
  email: string
  password: string
  phoneNumber: string
  role: Role
}

export type FieldErrors = Partial<Record<keyof SignupInput, string>>

// Go measures passwords with len(), which counts bytes. A password of accented
// or Vietnamese characters can pass a .length check here and still be rejected
// by the server, so count bytes the same way it does.
export function passwordByteLength(password: string): number {
  return new TextEncoder().encode(password).length
}

export function validate(input: SignupInput): FieldErrors {
  const errors: FieldErrors = {}

  if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(input.email.trim())) {
    errors.email = 'Enter a valid email address'
  }

  const bytes = passwordByteLength(input.password)
  if (bytes < MIN_PASSWORD_BYTES) {
    errors.password = `Must be at least ${MIN_PASSWORD_BYTES} characters`
  } else if (bytes > MAX_PASSWORD_BYTES) {
    errors.password = `Too long — ${MAX_PASSWORD_BYTES} characters maximum`
  }

  if (input.phoneNumber.trim() === '') {
    errors.phoneNumber = 'Phone number is required'
  }

  if (input.role !== 'player' && input.role !== 'owner') {
    errors.role = 'Choose an account type'
  }

  return errors
}
