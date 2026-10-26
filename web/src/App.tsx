import { useState } from 'react';
import { IdentityGate } from './identity';
import type { PulseSession } from './identity';
import { Dashboard } from './dashboard/Dashboard';

export default function App() {
  const [session, setSession] = useState<PulseSession | null>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('pulse-demo-session') || 'null') as PulseSession | null;
      return saved?.id && saved?.permissions ? saved : null;
    } catch {
      return null;
    }
  });

  const signIn = (next: PulseSession) => {
    const cleanSession = { ...next, permissions: next.permissions.filter(permission => String(permission) !== 'community') };
    localStorage.setItem('pulse-demo-session', JSON.stringify(cleanSession));
    setSession(cleanSession);
  };
  const signOut = () => {
    localStorage.removeItem('pulse-demo-session');
    setSession(null);
  };

  return session
    ? <Dashboard session={session} onSignOut={signOut} onSessionUpdate={signIn} />
    : <IdentityGate lang="en" onSignIn={signIn} />;
}
