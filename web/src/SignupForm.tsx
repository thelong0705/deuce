import { useState } from 'react'
import type { FormEvent } from 'react'

import { PasswordField } from './PasswordField'
import { PhoneField } from './PhoneField'
import { ApiError, login, signup } from './api'
import type { Session } from './api'
import { defaultCountry } from './countries'
import { MIN_PASSWORD_BYTES, validate } from './validation'
import type { FieldErrors, Role, SignupInput } from './validation'

const empty: SignupInput = {
  email: '',
  password: '',
  confirmPassword: '',
  countryISO: defaultCountry.iso,
  phoneNumber: '',
  role: 'player',
}

type Props = {
  // onSignedIn hands over the session the new account was signed in with.
  onSignedIn: (session: Session) => void
  // onNeedsSignIn is the fallback: the account exists but signing in failed,
  // so the sign-in form takes over with the email already filled.
  onNeedsSignIn: (email: string) => void
}

export function SignupForm({ onSignedIn, onNeedsSignIn }: Props) {
  const [input, setInput] = useState<SignupInput>(empty)
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

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
      await signup(input)
    } catch (err) {
      // A conflict is about one particular field, so it has to be routed by
      // code. Going on the 409 alone put "phone number already registered"
      // under the email.
      if (err instanceof ApiError && err.code === 'email_taken') {
        setFieldErrors({ email: err.message })
      } else if (err instanceof ApiError && err.code === 'phone_taken') {
        setFieldErrors({ phoneNumber: err.message })
      } else if (err instanceof ApiError) {
        setFormError(err.message)
      } else {
        setFormError('Something went wrong.')
      }
      setSubmitting(false)
      return
    }

    // Registering does not start a session, so signing up signs in too rather
    // than asking for the same password again on the next screen.
    try {
      onSignedIn(await login({ email: input.email, password: input.password }))
    } catch {
      // The account is real either way; only the session is missing, and the
      // sign-in form is the one screen that can say so honestly.
      onNeedsSignIn(input.email.trim())
    } finally {
      setSubmitting(false)
    }
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

      <PasswordField
        id="password"
        label="Password"
        autoComplete="new-password"
        value={input.password}
        error={fieldErrors.password}
        hint={`At least ${MIN_PASSWORD_BYTES} characters`}
        onChange={(password) => update({ password })}
      />

      <PasswordField
        id="confirm-password"
        label="Confirm password"
        autoComplete="new-password"
        value={input.confirmPassword}
        error={fieldErrors.confirmPassword}
        onChange={(confirmPassword) => update({ confirmPassword })}
      />

      <PhoneField
        countryISO={input.countryISO}
        phoneNumber={input.phoneNumber}
        error={fieldErrors.phoneNumber}
        onChange={update}
      />

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
