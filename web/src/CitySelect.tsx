import type { City } from './api'

type Props = {
  id: string
  label: string
  value: string
  cities: City[] | null
  error?: string
  onChange: (city: string) => void
}

export function CitySelect({ id, label, value, cities, error, onChange }: Props) {
  const loading = cities === null

  return (
    <>
      <label htmlFor={id}>
        {label}
        <select
          id={id}
          value={value}
          disabled={loading || cities.length === 0}
          onChange={(e) => onChange(e.target.value)}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? `${id}-error` : undefined}
        >
          <option value="">{placeholder(cities, error)}</option>
          {(cities ?? []).map((city) => (
            <option key={city.name} value={city.name}>
              {city.name}
            </option>
          ))}
        </select>
      </label>
      {error && (
        <p className="field-error" id={`${id}-error`}>
          {error}
        </p>
      )}
    </>
  )
}

function placeholder(cities: City[] | null, error?: string): string {
  if (error !== undefined) {
    return 'Unavailable'
  }
  if (cities === null) {
    return 'Loading…'
  }
  if (cities.length === 0) {
    return 'No cities available'
  }
  return 'Choose a city'
}
