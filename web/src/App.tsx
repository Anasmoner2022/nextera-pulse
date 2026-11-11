import { useState } from 'react';
import { IdentityGate } from './identity';
import type { PulseSession } from './identity';
import { Dashboard } from './dashboard/Dashboard';
import { usesLiveApi } from './data/students';

export default function App() {
  // Restore the signed-in 01Edu person. A leftover demo preview is not an account.
  const [session, setSession] = useState<PulseSession | null>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('pulse-demo-session') || 'null') as PulseSession | null;
      if (!saved?.id || !saved?.permissions) return null;
      if (usesLiveApi() && !saved.id.startsWith('01edu-')) return null;
      return saved;
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
