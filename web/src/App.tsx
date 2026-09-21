import { useState } from 'react'

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

  function signIn(next: Session) {
    saveSession(next)
    setSession(next)
  }

  function signOut() {
    clearSession()
    setSession(null)
    setTab('login')
  }

  return (
    <main className="page">
      <header className="brand">
        <span className="brand-mark">deuce</span>
        <span className="brand-sub">tennis court booking</span>
      </header>

      {session ? (
        <SignedIn session={session} onSignedOut={signOut} />
      ) : (
        <>
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
            <LoginForm key={email} initialEmail={email} onSignedIn={signIn} />
          ) : (
            <SignupForm
              onSignIn={(created) => {
                setEmail(created)
                setTab('login')
              }}
            />
          )}
        </>
      )}
    </main>
  )
}
