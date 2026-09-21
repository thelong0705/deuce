import type { Session, User } from './api'

type Props = {
  session: Session
  user: User | null
}

export function Account({ session, user }: Props) {
  return (
    <section className="card">
      <h1>Account</h1>
      <p className="lede">Your session is active.</p>

      <dl className="summary">
        <dt>Email</dt>
        <dd>{user ? user.email : <span className="mono">{session.user_id}</span>}</dd>
        <dt>Phone</dt>
        <dd>{user ? user.phone_number : '…'}</dd>
        <dt>Account</dt>
        <dd>{user ? accountLabel(user) : '…'}</dd>
        <dt>Signed in until</dt>
        <dd>{formatExpiry(session.expires_at)}</dd>
      </dl>
    </section>
  )
}

function accountLabel(user: User): string {
  return user.role === 'owner' ? 'Court owner' : 'Player'
}

function formatExpiry(value: string): string {
  const at = new Date(value)
  if (Number.isNaN(at.getTime())) {
    return value
  }

  return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
