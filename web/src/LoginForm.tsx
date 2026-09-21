import { useState } from 'react'
import type { FormEvent } from 'react'

import { ApiError, login } from './api'
import type { Session } from './api'
import { validateLogin } from './validation'
import type { LoginFieldErrors, LoginInput } from './validation'

type Props = {
  // initialEmail prefills the field after a fresh signup.
  initialEmail?: string
  onSignedIn: (session: Session) => void
}

export function LoginForm({ initialEmail = '', onSignedIn }: Props) {
  const [input, setInput] = useState<LoginInput>({ email: initialEmail, password: '' })
  const [fieldErrors, setFieldErrors] = useState<LoginFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function update(patch: Partial<LoginInput>) {
    setInput((prev) => ({ ...prev, ...patch }))
    setFieldErrors({})
    setFormError(null)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const errors = validateLogin(input)
    setFieldErrors(errors)
    if (Object.values(errors).some(Boolean)) {
      return
    }

    setSubmitting(true)
    setFormError(null)
    try {
      onSignedIn(await login(input))
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        // The server deliberately does not say which half was wrong, and
        // neither does this.
        setFormError('Email or password is incorrect.')
      } else if (err instanceof ApiError) {
        setFormError(err.message)
      } else {
        setFormError('Something went wrong.')
      }
      setInput((prev) => ({ ...prev, password: '' }))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form className="card" onSubmit={handleSubmit} noValidate>
      <h1>Sign in</h1>
      <p className="lede">Welcome back.</p>

      <label htmlFor="login-email">
        Email
        <input
          id="login-email"
          type="email"
          autoComplete="email"
          value={input.email}
          onChange={(e) => update({ email: e.target.value })}
          aria-invalid={Boolean(fieldErrors.email)}
          aria-describedby={fieldErrors.email ? 'login-email-error' : undefined}
        />
      </label>
      {fieldErrors.email && (
        <p className="field-error" id="login-email-error">
          {fieldErrors.email}
        </p>
      )}

      <label htmlFor="login-password">
        Password
        <input
          id="login-password"
          type="password"
          autoComplete="current-password"
          value={input.password}
          onChange={(e) => update({ password: e.target.value })}
          aria-invalid={Boolean(fieldErrors.password)}
          aria-describedby={fieldErrors.password ? 'login-password-error' : undefined}
        />
      </label>
      {fieldErrors.password && (
        <p className="field-error" id="login-password-error">
          {fieldErrors.password}
        </p>
      )}

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <button type="submit" disabled={submitting}>
        {submitting ? 'Signing in…' : 'Sign in'}
      </button>
    </form>
  )
}
