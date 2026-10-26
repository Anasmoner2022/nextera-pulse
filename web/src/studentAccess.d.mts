import type { PulseRole, PulseAccount } from './identity';
import type { StudentPaceRow } from './domain';

export function visibleStudentsForRole(role: PulseRole, rows: StudentPaceRow[], assignedLogins?: string[], studentLogin?: string): StudentPaceRow[];
export function assignStudentToMentor(mentors: PulseAccount[], mentorId: string, login: string, assigned: boolean): PulseAccount[];
