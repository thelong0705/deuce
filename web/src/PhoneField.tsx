import { countries } from './countries'

type Props = {
  countryISO: string
  phoneNumber: string
  error?: string
  onChange: (patch: { countryISO?: string; phoneNumber?: string }) => void
}

// A dialling-code select and the national number, drawn as one control. The
// select is a native one so it gets the platform's own picker on a phone and
// keyboard support everywhere; only its chrome is replaced.
export function PhoneField({ countryISO, phoneNumber, error, onChange }: Props) {
  return (
    <>
      <label htmlFor="phone">Phone number</label>

      <div className="phone-field" data-invalid={Boolean(error)}>
        <select
          className="phone-dial"
          aria-label="Country calling code"
          value={countryISO}
          onChange={(e) => onChange({ countryISO: e.target.value })}
        >
          {countries.map((country) => (
            <option key={country.iso} value={country.iso} title={country.name}>
              {country.flag} +{country.dial}
            </option>
          ))}
        </select>

        <input
          id="phone"
          className="phone-number"
          type="tel"
          autoComplete="tel-national"
          inputMode="numeric"
          placeholder="Nhập số điện thoại"
          value={phoneNumber}
          onChange={(e) => onChange({ phoneNumber: e.target.value })}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? 'phone-error' : undefined}
        />
      </div>

      {error && (
        <p className="field-error" id="phone-error">
          {error}
        </p>
      )}
    </>
  )
}
