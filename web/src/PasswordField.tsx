import { useState } from 'react'
import type { ReactNode } from 'react'

type Props = {
  id: string
  label: string
  value: string
  autoComplete: 'current-password' | 'new-password'
  placeholder?: string
  error?: string
  // hint shows under the field while there is no error.
  hint?: ReactNode
  onChange: (value: string) => void
}

export function PasswordField({
  id,
  label,
  value,
  autoComplete,
  placeholder,
  error,
  hint,
  onChange,
}: Props) {
  const [revealed, setRevealed] = useState(false)

  const describedBy = error ? `${id}-error` : hint ? `${id}-hint` : undefined

  return (
    <>
      <label htmlFor={id}>{label}</label>

      <div className="password-field" data-invalid={Boolean(error)}>
        <input
          id={id}
          type={revealed ? 'text' : 'password'}
          autoComplete={autoComplete}
          placeholder={placeholder}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          aria-invalid={Boolean(error)}
          aria-describedby={describedBy}
        />

        <button
          type="button"
          className="password-reveal"
          // The label says what the button will do, which is what a screen
          // reader user needs; aria-pressed says what state it is in.
          aria-label={revealed ? 'Hide password' : 'Show password'}
          aria-pressed={revealed}
          onClick={() => setRevealed((shown) => !shown)}
        >
          <EyeIcon crossed={!revealed} />
        </button>
      </div>

      {error ? (
        <p className="field-error" id={`${id}-error`}>
          {error}
        </p>
      ) : hint ? (
        <p className="hint" id={`${id}-hint`}>
          {hint}
        </p>
      ) : null}
    </>
  )
}

function EyeIcon({ crossed }: { crossed: boolean }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="20"
      height="20"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      <path d="M1.8 12S5.8 5 12 5s10.2 7 10.2 7-4 7-10.2 7S1.8 12 1.8 12Z" />
      <circle cx="12" cy="12" r="3" />
      {crossed && <path d="M4 20 20 4" />}
    </svg>
  )
}
