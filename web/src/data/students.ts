import type { StudentsResponse } from '../domain';
import { mockStudents } from './mockStudents';

export type FollowUp = {
  id: number;
  studentId: number;
  note: string;
  createdAt: string;
};

type InterventionPayload = {
  id: number;
  studentId: number;
  action: string;
  createdAt: string;
};

export function usesLiveApi(): boolean {
  return (import.meta.env.VITE_STUDENTS_SOURCE ?? 'api') === 'api';
}

function apiUrl(path: string): { url: string; credentials: RequestCredentials } {
  const baseUrl = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/$/, '');
  return { url: `${baseUrl}${path}`, credentials: baseUrl ? 'include' : 'same-origin' };
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) throw new Error(`API returned ${response.status}. Please try again.`);
  return await response.json() as T;
}

const DEMO_LATENCY_MS = 320;

// Keep mock and live records behind one adapter so UI components do not need
// source-specific branches or transport details.
function waitForDemoNetwork(): Promise<void> {
  return new Promise(resolve => window.setTimeout(resolve, DEMO_LATENCY_MS));
}

export async function getStudents(): Promise<StudentsResponse> {
  const source = import.meta.env.VITE_STUDENTS_SOURCE ?? 'api';

  if (source === 'mock') {
    await waitForDemoNetwork();
    // Return an API-shaped copy, so screens cannot accidentally mutate fixtures.
    return structuredClone(mockStudents);
  }

  if (source === 'api') {
    const request = apiUrl('/api/v1/cohorts/97/students');
    const response = await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials });
    return readJSON<StudentsResponse>(response);
  }

  throw new Error(`Unknown VITE_STUDENTS_SOURCE "${source}". Use "mock" or "api".`);
}

function asFollowUp(row: InterventionPayload): FollowUp {
  return { id: row.id, studentId: row.studentId, note: row.action, createdAt: row.createdAt };
}

export async function getFollowUps(): Promise<FollowUp[]> {
  const request = apiUrl('/api/v1/interventions');
  const body = await readJSON<{ data: InterventionPayload[] | null }>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
  return (body.data ?? []).map(asFollowUp);
}

export type ProgressBoard = {
  source: string;
  syncedAt?: string;
  projects: { id: number; name: string; path: string }[];
  students: { studentId: number; login: string; displayName: string; mentorName: string; done: boolean[]; completed: number; total: number }[];
};

export type StaffMember = {
  studentId: number;
  sourceUserId: number;
  login: string;
  displayName: string;
  email: string;
  sourceRole: string;
  pulseRole: 'manager' | 'mentor';
};

export type AcademyIdentity = {
  studentId: number;
  sourceUserId: number;
  login: string;
  displayName: string;
  email: string;
  sourceRole: string;
  pulseRole: 'manager' | 'mentor' | 'student';
};

export async function signInWithAcademyLogin(login: string, password: string): Promise<AcademyIdentity> {
  const request = apiUrl('/api/v1/session');
  const response = await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ login, password }),
  });
  if (response.status === 401) throw new Error('INVALID_CREDENTIALS');
  if (response.status === 404) throw new Error('NOT_IN_COHORT');
  if (!response.ok) throw new Error('SIGNIN_UNAVAILABLE');
  return readJSON<AcademyIdentity>(response);
}

export async function assignMentor(studentId: number, mentorId: number): Promise<void> {
  const request = apiUrl('/api/v1/mentor-assignments');
  await readJSON<{ studentId: number }>(await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ studentId, mentorId }),
  }));
}

export async function getStaff(): Promise<{ managers: StaffMember[]; mentors: StaffMember[] }> {
  const request = apiUrl('/api/v1/staff');
  const body = await readJSON<{ managers: StaffMember[] | null; mentors: StaffMember[] | null }>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
  return { managers: body.managers ?? [], mentors: body.mentors ?? [] };
}

export async function getCohortProgress(): Promise<ProgressBoard> {
  const request = apiUrl('/api/v1/cohorts/97/progress');
  return readJSON<ProgressBoard>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
}

export async function getStudentProgress(studentId: number): Promise<ProgressBoard> {
  const request = apiUrl(`/api/v1/students/${studentId}/progress`);
  return readJSON<ProgressBoard>(await fetch(request.url, { headers: { Accept: 'application/json' }, credentials: request.credentials }));
}

export async function createFollowUp(input: { studentId: number; note: string }): Promise<FollowUp> {
  const request = apiUrl('/api/v1/interventions');
  const body = await readJSON<{ data: InterventionPayload }>(await fetch(request.url, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    credentials: request.credentials,
    body: JSON.stringify({ studentId: input.studentId, action: input.note, reasonCode: 'MENTOR_FOLLOW_UP', type: 'follow-up' }),
  }));
  return asFollowUp(body.data);
}
