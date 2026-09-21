import { useState } from 'react'
import type { FormEvent } from 'react'

import { ApiError, signup } from './api'
import type { CreatedUser } from './api'
import { MAX_PASSWORD_BYTES, validate } from './validation'
import type { FieldErrors, Role, SignupInput } from './validation'

const empty: SignupInput = {
  email: '',
  password: '',
  phoneNumber: '',
  role: 'player',
}

type Props = {
  // onSignIn hands the new account's email to the sign-in form. Registering
  // does not start a session, so the user still has to log in.
  onSignIn: (email: string) => void
}

export function SignupForm({ onSignIn }: Props) {
  const [input, setInput] = useState<SignupInput>(empty)
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [created, setCreated] = useState<CreatedUser | null>(null)

  function update(patch: Partial<SignupInput>) {
    setInput((prev) => ({ ...prev, ...patch }))
    setFieldErrors({})
    setFormError(null)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const errors = validate(input)
    setFieldErrors(errors)
    if (Object.values(errors).some(Boolean)) {
      return
    }

    setSubmitting(true)
    setFormError(null)
    try {
      setCreated(await signup(input))
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setFieldErrors({ email: err.message })
      } else if (err instanceof ApiError) {
        setFormError(err.message)
      } else {
        setFormError('Something went wrong.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  if (created) {
    return (
      <div className="card">
        <h1>You&rsquo;re in</h1>
        <p className="lede">
          Welcome, <strong>{created.display_name}</strong>.
        </p>
        <dl className="summary">
          <dt>Email</dt>
          <dd>{created.email}</dd>
          <dt>Account</dt>
          <dd>{created.role === 'owner' ? 'Court owner' : 'Player'}</dd>
        </dl>
        <button type="button" onClick={() => onSignIn(created.email)}>
          Sign in
        </button>
        <button
          type="button"
          className="secondary quiet"
          onClick={() => {
            setCreated(null)
            setInput(empty)
          }}
        >
          Create another account
        </button>
      </div>
    )
  }

  return (
    <form className="card" onSubmit={handleSubmit} noValidate>
      <h1>Create your account</h1>
      <p className="lede">Book a court in under a minute.</p>

      <label htmlFor="email">
        Email
        <input
          id="email"
          type="email"
          autoComplete="email"
          value={input.email}
          onChange={(e) => update({ email: e.target.value })}
          aria-invalid={Boolean(fieldErrors.email)}
          aria-describedby={fieldErrors.email ? 'email-error' : undefined}
        />
      </label>
      {fieldErrors.email && (
        <p className="field-error" id="email-error">
          {fieldErrors.email}
        </p>
      )}

      <label htmlFor="password">
        Password
        <input
          id="password"
          type="password"
          autoComplete="new-password"
          value={input.password}
          onChange={(e) => update({ password: e.target.value })}
          aria-invalid={Boolean(fieldErrors.password)}
          aria-describedby={fieldErrors.password ? 'password-error' : 'password-hint'}
        />
      </label>
      {fieldErrors.password ? (
        <p className="field-error" id="password-error">
          {fieldErrors.password}
        </p>
      ) : (
        <p className="hint" id="password-hint">
          {input.password.length} / {MAX_PASSWORD_BYTES} characters
        </p>
      )}

      <label htmlFor="phone">
        Phone number
        <input
          id="phone"
          type="tel"
          autoComplete="tel"
          placeholder="+84901234567"
          value={input.phoneNumber}
          onChange={(e) => update({ phoneNumber: e.target.value })}
          aria-invalid={Boolean(fieldErrors.phoneNumber)}
          aria-describedby={fieldErrors.phoneNumber ? 'phone-error' : undefined}
        />
      </label>
      {fieldErrors.phoneNumber && (
        <p className="field-error" id="phone-error">
          {fieldErrors.phoneNumber}
        </p>
      )}

      <label htmlFor="role">
        Account type
        <select
          id="role"
          value={input.role}
          onChange={(e) => update({ role: e.target.value as Role })}
        >
          <option value="player">Player &mdash; I want to book courts</option>
          <option value="owner">Court owner &mdash; I want to list courts</option>
        </select>
      </label>

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <button type="submit" disabled={submitting}>
        {submitting ? 'Creating account…' : 'Create account'}
      </button>
    </form>
  )
}
