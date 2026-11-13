import type { AcademyIdentity } from '../data/students';
import type { PulseSession } from './types';
import { defaultPermissions } from './defaults';

/**
 * Builds the in-memory session for a person the API signed in. The role comes
 * from the server; permissions here only shape the UI, the API enforces access.
 */
export function sessionFromIdentity(person: AcademyIdentity): PulseSession {
  return {
    id: `01edu-${person.pulseRole}-${person.studentId}`,
    name: person.displayName || person.login,
    email: person.email,
    role: person.pulseRole,
    permissions: defaultPermissions[person.pulseRole],
    studentLogin: person.login,
  };
}
