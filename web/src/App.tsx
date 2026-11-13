import { useEffect, useState } from 'react';
import { IdentityGate } from './identity';
import type { PulseSession } from './identity';
import { sessionFromIdentity } from './identity/fromAcademy';
import { Dashboard } from './dashboard/Dashboard';
import { UNAUTHORIZED_EVENT, getCurrentSession, signOutSession, usesLiveApi } from './data/students';

const demoSessionKey = 'pulse-demo-session';

function readDemoSession(): PulseSession | null {
  try {
    const saved = JSON.parse(localStorage.getItem(demoSessionKey) || 'null') as PulseSession | null;
    return saved?.id && saved?.permissions ? saved : null;
  } catch {
    return null;
  }
}

export default function App() {
  const live = usesLiveApi();
  // Live mode: identity comes only from the API's HttpOnly session cookie
  // (GET /api/v1/session). Nothing about who is signed in is kept in the
  // browser. Demo mode (mock data) keeps its preview session locally.
  const [session, setSession] = useState<PulseSession | null>(() => live ? null : readDemoSession());
  const [checking, setChecking] = useState(live);

  useEffect(() => {
    if (!live) return;
    localStorage.removeItem(demoSessionKey); // drop identities older builds stored
    let active = true;
    getCurrentSession()
      .then(person => { if (active) setSession(person ? sessionFromIdentity(person) : null); })
      .catch(() => { if (active) setSession(null); })
      .finally(() => { if (active) setChecking(false); });
    const onUnauthorized = () => setSession(null);
    window.addEventListener(UNAUTHORIZED_EVENT, onUnauthorized);
    return () => { active = false; window.removeEventListener(UNAUTHORIZED_EVENT, onUnauthorized); };
  }, [live]);

  const signIn = (next: PulseSession) => {
    const cleanSession = { ...next, permissions: next.permissions.filter(permission => String(permission) !== 'community') };
    if (!live) localStorage.setItem(demoSessionKey, JSON.stringify(cleanSession));
    setSession(cleanSession);
  };
  const signOut = () => {
    setSession(null);
    if (live) void signOutSession();
    else localStorage.removeItem(demoSessionKey);
  };

  if (checking) return <main className="auth-page" aria-busy="true"><p className="auth-notice" role="status">Checking your session…</p></main>;
  return session
    ? <Dashboard session={session} onSignOut={signOut} onSessionUpdate={signIn} />
    : <IdentityGate lang="en" onSignIn={signIn} />;
}
