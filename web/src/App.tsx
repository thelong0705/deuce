import { useCallback, useState } from 'react'

import { LoginForm } from './LoginForm'
import { SignedIn } from './SignedIn'
import { SignupForm } from './SignupForm'
import type { Session } from './api'
import { clearSession, loadSession, saveSession } from './session'

type Tab = 'login' | 'signup'

export function App() {
  const [session, setSession] = useState<Session | null>(loadSession)
  const [tab, setTab] = useState<Tab>('login')
  const [email, setEmail] = useState('')
  // Set only when signing up created the account but could not start the
  // session, so the sign-in form can explain why it is being asked again.
  const [notice, setNotice] = useState<string | null>(null)

  function signIn(next: Session) {
    saveSession(next)
    setSession(next)
  }

  // Stable identity: SignedIn takes this as an effect dependency, and a fresh
  // function every render would refetch /me on every render.
  const signOut = useCallback(() => {
    clearSession()
    setSession(null)
    setTab('login')
  }, [])

  // Signed in, the brand moves into the menu bar and SignedIn owns the whole
  // page, so there is no shell to share with the signed-out screens.
  if (session) {
    return <SignedIn session={session} onSignedOut={signOut} />
  }

  return (
    <main className="page">
      <header className="brand">
        <span className="brand-mark">deuce</span>
        <span className="brand-sub">tennis court booking</span>
      </header>

      <nav className="tabs" aria-label="Account">
        <button
          type="button"
          className="tab"
          aria-current={tab === 'login'}
          onClick={() => setTab('login')}
        >
          Sign in
        </button>
        <button
          type="button"
          className="tab"
          aria-current={tab === 'signup'}
          onClick={() => setTab('signup')}
        >
          Create account
        </button>
      </nav>

      {tab === 'login' ? (
        // Remounting on email change lets a fresh signup prefill the field.
        <LoginForm key={email} initialEmail={email} notice={notice} onSignedIn={signIn} />
      ) : (
        <SignupForm
          onSignedIn={signIn}
          onNeedsSignIn={(created) => {
            setEmail(created)
            setNotice('Your account is ready. Sign in to continue.')
            setTab('login')
          }}
        />
      )}
    </main>
  )
}
