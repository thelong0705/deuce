import type { User } from './api'

export type MenuItem<K extends string> = {
  key: K
  label: string
}

type Props<K extends string> = {
  user: User | null
  items: MenuItem<K>[]
  active: K
  onSelect: (key: K) => void
  onSignOut: () => void
  signingOut: boolean
}

export function MenuBar<K extends string>({
  user,
  items,
  active,
  onSelect,
  onSignOut,
  signingOut,
}: Props<K>) {
  return (
    <header className="menubar">
      <div className="menubar-inner">
        <span className="brand-mark">deuce</span>

        {/* The items depend on the role, so there is nothing to show until
            /me has answered. An empty nav beats one that changes under the
            cursor a moment later. */}
        <nav className="menu" aria-label="Sections">
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              className="menu-item"
              aria-current={item.key === active ? 'page' : undefined}
              onClick={() => onSelect(item.key)}
            >
              {item.label}
            </button>
          ))}
        </nav>

        <div className="menubar-account">
          {user && <span className="menubar-email">{user.email}</span>}
          <button type="button" className="menu-signout" onClick={onSignOut} disabled={signingOut}>
            {signingOut ? 'Signing out…' : 'Sign out'}
          </button>
        </div>
      </div>
    </header>
  )
}
