import { useId, useState } from 'react'
import type { FormEvent } from 'react'

import { ApiError, createCourt } from './api'
import type { Court } from './api'
import { validateCourt } from './validation'
import type { CourtFieldErrors, CourtInput } from './validation'

const empty: CourtInput = {
  name: '',
  openHour: '6',
  closeHour: '22',
  pricePerHour: '',
}

// Opening can be any hour of the day; closing runs to 24, midnight.
const openHours = Array.from({ length: 24 }, (_, hour) => hour)
const closeHours = Array.from({ length: 24 }, (_, i) => i + 1)

type Props = {
  venueID: string
  onCreated: (court: Court) => void
  onUnauthorized: () => void
}

export function CourtForm({ venueID, onCreated, onUnauthorized }: Props) {
  const [input, setInput] = useState<CourtInput>(empty)
  const [fieldErrors, setFieldErrors] = useState<CourtFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // Several of these forms can be open at once, one per venue, so the ids
  // cannot be literals.
  const id = useId()

  function update(patch: Partial<CourtInput>) {
    setInput((prev) => ({ ...prev, ...patch }))
    setFieldErrors({})
    setFormError(null)
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()

    const errors = validateCourt(input)
    setFieldErrors(errors)
    if (Object.values(errors).some(Boolean)) {
      return
    }

    setSubmitting(true)
    setFormError(null)
    try {
      onCreated(await createCourt(venueID, input))
      setInput(empty)
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        onUnauthorized()
      } else if (err instanceof ApiError && err.code === 'court_name_taken') {
        setFieldErrors({ name: 'This venue already has a court with that name' })
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
    <form className="court-form" onSubmit={handleSubmit} noValidate>
      <label htmlFor={`${id}-name`}>
        Court name
        <input
          id={`${id}-name`}
          value={input.name}
          onChange={(e) => update({ name: e.target.value })}
          aria-invalid={Boolean(fieldErrors.name)}
          aria-describedby={fieldErrors.name ? `${id}-name-error` : undefined}
        />
      </label>
      {fieldErrors.name && (
        <p className="field-error" id={`${id}-name-error`}>
          {fieldErrors.name}
        </p>
      )}

      <div className="field-row">
        <label htmlFor={`${id}-open`}>
          Opens
          <select
            id={`${id}-open`}
            value={input.openHour}
            onChange={(e) => update({ openHour: e.target.value })}
          >
            {openHours.map((hour) => (
              <option key={hour} value={hour}>
                {formatHour(hour)}
              </option>
            ))}
          </select>
        </label>

        <label htmlFor={`${id}-close`}>
          Closes
          <select
            id={`${id}-close`}
            value={input.closeHour}
            onChange={(e) => update({ closeHour: e.target.value })}
            aria-invalid={Boolean(fieldErrors.closeHour)}
            aria-describedby={fieldErrors.closeHour ? `${id}-close-error` : undefined}
          >
            {closeHours.map((hour) => (
              <option key={hour} value={hour}>
                {formatHour(hour)}
              </option>
            ))}
          </select>
        </label>
      </div>
      {fieldErrors.closeHour && (
        <p className="field-error" id={`${id}-close-error`}>
          {fieldErrors.closeHour}
        </p>
      )}

      <label htmlFor={`${id}-price`}>
        Price per hour
        <input
          id={`${id}-price`}
          type="number"
          min="0"
          step="1"
          inputMode="numeric"
          value={input.pricePerHour}
          onChange={(e) => update({ pricePerHour: e.target.value })}
          aria-invalid={Boolean(fieldErrors.pricePerHour)}
          aria-describedby={fieldErrors.pricePerHour ? `${id}-price-error` : undefined}
        />
      </label>
      {fieldErrors.pricePerHour && (
        <p className="field-error" id={`${id}-price-error`}>
          {fieldErrors.pricePerHour}
        </p>
      )}

      {formError && (
        <p className="form-error" role="alert">
          {formError}
        </p>
      )}

      <button type="submit" disabled={submitting}>
        {submitting ? 'Adding…' : 'Add court'}
      </button>
    </form>
  )
}

// 24 is midnight at the end of the day, which "00:00" would read as the start.
export function formatHour(hour: number): string {
  return hour === 24 ? '24:00' : `${String(hour).padStart(2, '0')}:00`
}
