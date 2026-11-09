import type { StudentPaceRow } from './domain';
import type { PulseAccount } from './identity';

export function findLinkedStudentAccount(
  student: Pick<StudentPaceRow, 'login'>,
  accounts: PulseAccount[],
): PulseAccount | null;

export function findAssignedMentor(
  student: Pick<StudentPaceRow, 'login'>,
  accounts: PulseAccount[],
  mentors: PulseAccount[],
): PulseAccount | null;
