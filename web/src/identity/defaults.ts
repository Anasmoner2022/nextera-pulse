import type { PulseRole, PulseSession, AccessKey } from './types';
import { readAccounts } from './storage';

export const accessKeys: AccessKey[] = ['viewProgress', 'viewCohort', 'manageStudents', 'manageAccounts', 'manageInterventions', 'viewAllStudents', 'viewMentorPerformance'];

export const defaultPermissions: Record<PulseRole, AccessKey[]> = { manager: accessKeys, mentor: ['viewProgress', 'viewCohort', 'manageStudents', 'manageAccounts', 'manageInterventions'], student: ['viewProgress'] };

export const demoSession = (role: PulseRole): PulseSession => {
  const id = role === 'manager' ? 'demo-manager' : role === 'mentor' ? 'demo-mentor' : 'demo-student';
  const saved = readAccounts().find(account => account.id === id);
  if (saved) { const { passwordHash: _passwordHash, ...session } = saved; return session ;}
  return role === 'manager'
    ? { id, name: 'Program Manager', email: 'manager@nextera.io', role, permissions: defaultPermissions.manager, quote: 'A clear plan helps every learner grow.' }
    : role === 'mentor'
      ? { id, name: 'Mentor demo', email: 'mentor@nextera.io', role, permissions: defaultPermissions.mentor, quote: 'Every learner deserves a clear next step.' }
      : { id, name: 'Malak Hassan', email: 'malak.hassan@example.com', role, permissions: defaultPermissions.student, studentLogin: 'malak.hassan', quote: 'Small steps, big progress.' };
};
