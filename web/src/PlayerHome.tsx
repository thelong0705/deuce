import { useState } from 'react'

import { Browse } from './Browse'
import { MyBookings } from './MyBookings'

type Props = {
  onUnauthorized: () => void
}

export function PlayerHome({ onUnauthorized }: Props) {
  // Bumped after a booking so the list below reloads from the server rather
  // than being patched locally from the create response.
  const [version, setVersion] = useState(0)

  return (
    <>
      <Browse onBooked={() => setVersion((v) => v + 1)} onUnauthorized={onUnauthorized} />
      <MyBookings version={version} onUnauthorized={onUnauthorized} />
    </>
  )
}
