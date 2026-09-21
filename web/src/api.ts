import type { Role, SignupInput } from './validation'

export type CreatedUser = {
  id: string
  email: string
  display_name: string
  role: Role
  created_at: string
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function signup(input: SignupInput): Promise<CreatedUser> {
  let response: Response
  try {
    response = await fetch('/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email: input.email.trim(),
        password: input.password,
        phone_number: input.phoneNumber.trim(),
        role: input.role,
      }),
    })
  } catch {
    throw new ApiError(0, 'Could not reach the server. Is it running?')
  }

  if (!response.ok) {
    throw new ApiError(response.status, await errorMessage(response))
  }

  return (await response.json()) as CreatedUser
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const body: unknown = await response.json()
    if (
      typeof body === 'object' &&
      body !== null &&
      'error' in body &&
      typeof (body as { error: unknown }).error === 'string'
    ) {
      return (body as { error: string }).error
    }
  } catch {
    // Not JSON; fall through to the generic message.
  }

  if (response.status === 404) {
    return 'The signup endpoint does not exist yet.'
  }
  return `Request failed (${response.status})`
}
