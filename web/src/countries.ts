export type Country = {
  // iso is the two-letter code, used only as a stable key.
  iso: string
  name: string
  dial: string
  flag: string
}

// A short list, Vietnam first because that is where the courts are. Adding a
// country is one line; nothing here is generated.
//
// Every one of these drops a single leading trunk zero when the number is
// normalised. That is wrong for Italy, among others, so a country whose zero
// is part of the national number needs more than a new row.
export const countries: Country[] = [
  { iso: 'VN', name: 'Vietnam', dial: '84', flag: '🇻🇳' },
  { iso: 'SG', name: 'Singapore', dial: '65', flag: '🇸🇬' },
  { iso: 'TH', name: 'Thailand', dial: '66', flag: '🇹🇭' },
  { iso: 'MY', name: 'Malaysia', dial: '60', flag: '🇲🇾' },
  { iso: 'ID', name: 'Indonesia', dial: '62', flag: '🇮🇩' },
  { iso: 'PH', name: 'Philippines', dial: '63', flag: '🇵🇭' },
  { iso: 'JP', name: 'Japan', dial: '81', flag: '🇯🇵' },
  { iso: 'KR', name: 'South Korea', dial: '82', flag: '🇰🇷' },
  { iso: 'CN', name: 'China', dial: '86', flag: '🇨🇳' },
  { iso: 'AU', name: 'Australia', dial: '61', flag: '🇦🇺' },
  { iso: 'GB', name: 'United Kingdom', dial: '44', flag: '🇬🇧' },
  { iso: 'US', name: 'United States', dial: '1', flag: '🇺🇸' },
]

export const defaultCountry = countries[0]

export function countryByISO(iso: string): Country {
  return countries.find((c) => c.iso === iso) ?? defaultCountry
}
