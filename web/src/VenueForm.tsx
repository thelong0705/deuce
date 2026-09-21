import { useState } from 'react'
import type { FormEvent } from 'react'

import { ApiError, createVenue } from './api'
import type { Venue } from './api'
import { validateVenue } from './validation'
import type { VenueFieldErrors, VenueInput } from './validation'

const empty: VenueInput = { name: '', city: '', address: '' }

type Props = {
  onCreated: (venue: Venue) => void
  onUnauthorized: () => void
}

export function VenueForm({ onCreated, onUnauthorized }: Props) {
  const [input, setInput] = useState<VenueInput>(empty)
  const [fieldErrors, setFieldErrors] = useState<VenueFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  function update(patch: Partial<VenueInput>) {
    setInput((prev) => ({ ...prev, ...patch }))
    setFieldErrors({})
    setFormError(null)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const errors = validateVenue(input)
    setFieldErrors(errors)
    if (Object.values(errors).some(Boolean)) {
      return
    }

    setSubmitting(true)
    setFormError(null)
    try {
      onCreated(await createVenue(input))
      setInput(empty)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
      } else if (err instanceof ApiError && err.code === 'not_an_owner') {
        setFormError('This account is a player. Only court owners can register venues.')
      } else if (err instanceof ApiError) {
        setFormError(err.message)
      } else {
        setFormError('Something went wrong.')
      }
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} noValidate>
      <label htmlFor="venue-name">
        Venue name
        <input
          id="venue-name"
          value={input.name}
          onChange={(e) => update({ name: e.target.value })}
          aria-invalid={Boolean(fieldErrors.name)}
          aria-describedby={fieldErrors.name ? 'venue-name-error' : undefined}
        />
      </label>
      {fieldErrors.name && (
        <p className="field-error" id="venue-name-error">
          {fieldErrors.name}
        </p>
      )}

      <label htmlFor="venue-city">
        City
        <input
          id="venue-city"
          autoComplete="address-level2"
          value={input.city}
          onChange={(e) => update({ city: e.target.value })}
          aria-invalid={Boolean(fieldErrors.city)}
          aria-describedby={fieldErrors.city ? 'venue-city-error' : undefined}
        />
      </label>
      {fieldErrors.city && (
        <p className="field-error" id="venue-city-error">
          {fieldErrors.city}
        </p>
      )}

      <label htmlFor="venue-address">
        Address
        <input
          id="venue-address"
          autoComplete="street-address"
          value={input.address}
          onChange={(e) => update({ address: e.target.value })}
          aria-invalid={Boolean(fieldErrors.address)}
          aria-describedby={fieldErrors.address ? 'venue-address-error' : undefined}
        />
      </label>
      {fieldErrors.address && (
        <p className="field-error" id="venue-address-error">
          {fieldErrors.address}
        </p>
      )}

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <button type="submit" disabled={submitting}>
        {submitting ? 'Registering…' : 'Register venue'}
      </button>
    </form>
  )
}
