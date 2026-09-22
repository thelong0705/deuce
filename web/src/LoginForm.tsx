import { useState } from 'react'
import type { FormEvent } from 'react'

import { PasswordField } from './PasswordField'
import { ApiError, login } from './api'
import type { Session } from './api'
import { validateLogin } from './validation'
import type { LoginFieldErrors, LoginInput } from './validation'

type Props = {
  // initialEmail prefills the field after a fresh signup.
  initialEmail?: string
  // notice explains why this form is being shown, when something sent the
  // user here rather than them choosing it.
  notice?: string | null
  onSignedIn: (session: Session) => void
}

export function LoginForm({ initialEmail = '', notice, onSignedIn }: Props) {
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

      {notice && (
        <p className="notice" role="status">
          {notice}
        </p>
      )}

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

      <PasswordField
        id="login-password"
        label="Password"
        autoComplete="current-password"
        value={input.password}
        error={fieldErrors.password}
        onChange={(password) => update({ password })}
      />

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
