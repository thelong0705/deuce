import type { LoginInput, Role, SignupInput } from './validation'

export type CreatedUser = {
  id: string
  email: string
  display_name: string
  role: Role
  created_at: string
}

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
      phone_number: input.phoneNumber.trim(),
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
