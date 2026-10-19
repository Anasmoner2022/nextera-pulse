import type { StudentPaceRow } from './domain';

export function createDemoStudentRecord(input: { studentId: number; login: string; displayName: string; enrolledAt?: string; }): StudentPaceRow;
export function syncStudentIdentity(row: StudentPaceRow, input: { login: string; displayName: string; }): StudentPaceRow;
